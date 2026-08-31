package motion

import (
	"fmt"
	"math"
	"sort"
)

// CellActivity is one part of the frame and the stretches in which it was busy
// while the rest of the frame was not.
//
// It is the spatial index the tool otherwise only draws, and an agent that
// cannot look at the picture has no other way to ask where something happened.
//
// Only local activity is listed. A cell busy at the moment everything else was
// busy locates nothing — a page navigation lights all forty-eight of them
// equally — so frame-wide stretches are reported once, separately, as what
// they are.
type CellActivity struct {
	Cell string `json:"cell"`
	Box  []int  `json:"box_xyxy"`
	// Share is how much of the analysed interval this cell was locally busy
	// for. It is the field to sort and filter on: a cell busy 90% of the time
	// is a spinner or a video, one busy 2% of it is a single change.
	Share   float64     `json:"busy_share"`
	Seconds float64     `json:"busy_seconds"`
	Ranges  [][]float64 `json:"ranges"`
	Omitted int         `json:"ranges_omitted,omitempty"`
	Peak    float64     `json:"peak_changed_fraction"`
	PeakAt  float64     `json:"peak_seconds"`
}

// Span is a stretch of time, used for activity that has no one place.
type Span struct {
	Start float64 `json:"start_seconds"`
	End   float64 `json:"end_seconds"`
}

// maxRanges bounds one cell's line. A cell flickering hundreds of times is
// described well enough by its share and its peak; the individual blinks are
// not what a reader is narrowing down with.
const maxRanges = 12

// Activity lists the parts of the frame that were busy on their own, busiest
// first, and separately the stretches in which the frame moved as a whole.
//
// The grid is coarse on purpose. It is an index for narrowing down — "the top
// centre was busy for half the recording and nothing else moved at all" — not
// a measurement, and the events carry the exact regions.
func (a *Analyzer) Activity(opt TimelineOptions) ([]CellActivity, []Span) {
	opt = opt.withDefaults()
	if len(a.samples) == 0 {
		return nil, nil
	}
	from, to := a.Span()
	floors := a.cellFloors(opt.MinFloor)
	gap := a.gapSamples(opt.MergeGap, opt.FPS)

	// Where the frame moved as a whole. A cell busy at that moment locates
	// nothing, so those samples are held back from the cells and reported once,
	// separately, as what they are.
	wide := make([]bool, len(a.samples))
	for i := range a.samples {
		lit := 0
		for c := range floors {
			if a.cellFraction(i, c, fast) > floors[c] {
				lit++
			}
		}
		wide[i] = float64(lit) >= frameWideArea*float64(len(floors))
	}

	// The segmentation the timeline uses, masked differently: it hides the
	// samples a cut has already claimed, this hides the ones where everything
	// moved at once. Sharing the pass is what keeps the two commands agreeing
	// about where something happened — written out twice, the definition of a
	// busy cell could drift between them with nothing failing to compile.
	out := make([]CellActivity, 0, len(floors))
	spans := a.fastSpans(floors, wide, gap)
	for start := 0; start < len(spans); {
		end := start
		for end < len(spans) && spans[end].cell == spans[start].cell {
			end++
		}
		// A run of one sample is not a stretch. Holding the frame-wide moments
		// back leaves a sample of residue at each edge of them, and a cell
		// described as busy for zero seconds is worse than no line at all.
		if kept := lasting(stretches(spans[start:end])); len(kept) > 0 {
			out = append(out, a.cellActivity(spans[start].cell, kept, to-from, opt))
		}
		start = end
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Share != out[j].Share {
			return out[i].Share > out[j].Share
		}
		return out[i].Peak > out[j].Peak
	})

	// An empty list, not a nil one: "the frame never moved as a whole" is an
	// answer, and a reader must not have to tell it apart from the field being
	// missing.
	frameWide := []Span{}
	for _, sp := range runs(len(a.samples), func(i int) bool { return wide[i] }, gap) {
		frameWide = append(frameWide, Span{
			Start: round2(a.samples[sp.from].Time), End: round2(a.samples[sp.to].Time),
		})
	}
	return out, frameWide
}

// stretches drops the cell each span belongs to, which the caller already knows.
func stretches(cells []cellSpan) []span {
	out := make([]span, len(cells))
	for i, c := range cells {
		out[i] = c.span
	}
	return out
}

func lasting(spans []span) []span {
	out := spans[:0]
	for _, sp := range spans {
		if sp.to > sp.from {
			out = append(out, sp)
		}
	}
	return out
}

func (a *Analyzer) cellActivity(c int, spans []span, total float64, opt TimelineOptions) CellActivity {
	box := a.sourceRect(a.grid.Bounds(c), opt)
	out := CellActivity{
		Cell: fmt.Sprintf("r%dc%d", c/a.grid.Cols, c%a.grid.Cols),
		Box:  []int{box[0], box[1], box[2], box[3]},
	}
	for _, sp := range spans {
		start, end := a.samples[sp.from].Time, a.samples[sp.to].Time
		out.Seconds += end - start
		if len(out.Ranges) < maxRanges {
			out.Ranges = append(out.Ranges, []float64{round2(start), round2(end)})
		} else {
			out.Omitted++
		}
		for i := sp.from; i <= sp.to; i++ {
			if v := a.cellFraction(i, c, fast); v > out.Peak {
				out.Peak, out.PeakAt = v, a.samples[i].Time
			}
		}
	}
	out.Seconds, out.Peak, out.PeakAt = round2(out.Seconds), round4(out.Peak), round2(out.PeakAt)
	if total > 0 {
		out.Share = round2(math.Min(1, out.Seconds/total))
	}
	return out
}
