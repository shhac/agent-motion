package motion

import (
	"image"
	"math"
	"slices"
)

// TimelineOptions tunes segmentation. Zero values take documented defaults.
type TimelineOptions struct {
	FPS                       float64
	SourceWidth, SourceHeight int
	DriftSeconds              float64
	CutFraction               float64
	MergeGap                  float64
	MinFloor                  float64
	MaxEvents                 int
}

func (o TimelineOptions) withDefaults() TimelineOptions {
	if o.CutFraction <= 0 {
		o.CutFraction = 0.5
	}
	if o.MergeGap <= 0 {
		o.MergeGap = 0.25
	}
	if o.MinFloor <= 0 {
		o.MinFloor = 0.0015
	}
	if o.MaxEvents <= 0 {
		o.MaxEvents = 40
	}
	return o
}

type span struct{ from, to int } // inclusive sample indices

func (s span) length() int { return s.to - s.from + 1 }

// cellSpan is one cell being active over one stretch of time.
type cellSpan struct {
	cell int
	span
	scale timescale
}

// group is a set of touching cells active over the same stretch, which is what
// becomes a single event.
type group struct {
	cells []int
	span
	scale timescale
}

func (a *Analyzer) gapSamples(gap, fps float64) int {
	if fps <= 0 {
		return 1
	}
	return max(1, int(math.Round(gap*fps)))
}

// cellFloors estimates per-cell noise. A small event fills a much larger share
// of one cell than of the whole frame, which is why sensitivity improves when
// the floor is local.
func (a *Analyzer) cellFloors(minimum float64) []float64 {
	floors := make([]float64, len(a.grid.Pixels))
	series := make([]float64, len(a.samples))
	for c := range floors {
		for i, s := range a.samples {
			series[i] = float64(s.Cells[c].Count(fast)) / float64(a.grid.Pixels[c])
		}
		// Two changed pixels is the smallest thing worth calling an event; one
		// is indistinguishable from a single noisy pixel.
		floors[c] = math.Max(robustFloor(series, minimum), 2/float64(a.grid.Pixels[c]))
	}
	return floors
}

func (a *Analyzer) fastSpans(floors []float64, consumed []bool, gap int) []cellSpan {
	var out []cellSpan
	for c := range floors {
		for _, sp := range runs(len(a.samples), func(i int) bool {
			return !consumed[i] && a.cellFraction(i, c, fast) > floors[c]
		}, gap) {
			out = append(out, cellSpan{cell: c, span: sp})
		}
	}
	return out
}

// slowSpans finds cells drifting without any fast activity of their own. The
// mask is per cell, so a constantly animating corner no longer hides a slow
// change happening elsewhere.
func (a *Analyzer) slowSpans(floors []float64, consumed []bool, gap int, opt TimelineOptions) []cellSpan {
	if opt.DriftSeconds <= 0 || opt.FPS <= 0 {
		return nil
	}
	lag := int(math.Round(opt.DriftSeconds * opt.FPS))
	var out []cellSpan
	for c := range floors {
		// Drift stays elevated for one window after any real change, so a
		// sample is masked when the most recent activity is within the window.
		masked := make([]bool, len(a.samples))
		recent := -1
		for i := range a.samples {
			if consumed[i] || a.cellFraction(i, c, fast) > floors[c] {
				recent = i
			}
			masked[i] = recent >= 0 && i-recent <= lag
		}
		for _, sp := range runs(len(a.samples), func(i int) bool {
			return !masked[i] && a.cellFraction(i, c, slow) > floors[c]
		}, lag) {
			if sp.to-sp.from+1 >= max(2, lag) {
				out = append(out, cellSpan{cell: c, span: sp, scale: slow})
			}
		}
	}
	return out
}

// cellFraction is the share of one cell that differed, on one timescale.
func (a *Analyzer) cellFraction(i, c int, t timescale) float64 {
	return float64(a.samples[i].Cells[c].Count(t)) / float64(a.grid.Pixels[c])
}

// groupSpans merges touching cells whose active stretches overlap, so one thing
// happening across several cells is one event and two things happening at once
// in different places stay two.
func groupSpans(spans []cellSpan, grid Grid, gap int) []group {
	sets := newDisjoint(len(spans))
	for i := range spans {
		for j := i + 1; j < len(spans); j++ {
			if mergeable(spans[i], spans[j], grid, gap) {
				sets.union(i, j)
			}
		}
	}
	find := sets.find
	byRoot := map[int]*group{}
	var order []int
	for i, s := range spans {
		root := find(i)
		g, ok := byRoot[root]
		if !ok {
			g = &group{span: span{from: s.from, to: s.to}, scale: s.scale}
			byRoot[root] = g
			order = append(order, root)
		}
		g.cells = append(g.cells, s.cell)
		g.from, g.to = min(g.from, s.from), max(g.to, s.to)
	}
	out := make([]group, 0, len(order))
	for _, root := range order {
		g := byRoot[root]
		slices.Sort(g.cells)
		g.cells = slices.Compact(g.cells)
		out = append(out, *g)
	}
	return out
}

// lengthRatio bounds how differently two stretches may last and still be called
// the same event. Without it, anything brief that happens next to something
// long-running is absorbed by it: a caption dropping six pixels for two frames
// merges into a progress bar that has been advancing for twenty seconds, and
// disappears. Cells belong to one event when they are active *together*, not
// when one is active while the other happens to be running.
const lengthRatio = 8

// mergeable reports whether two cell stretches belong to the same event: same
// timescale, touching cells, overlapping in time, and of comparable duration.
func mergeable(a, b cellSpan, grid Grid, gap int) bool {
	if a.scale != b.scale || !grid.Adjacent(a.cell, b.cell) || !overlaps(a.span, b.span, gap) {
		return false
	}
	shorter, longer := a.length(), b.length()
	if shorter > longer {
		shorter, longer = longer, shorter
	}
	return longer <= lengthRatio*shorter
}

func overlaps(a, b span, gap int) bool {
	return a.from-gap <= b.to && b.from-gap <= a.to
}

// disjoint is the union-find behind grouping. It is separated so groupSpans
// reads as "merge what touches, then collect", which is its actual job.
type disjoint []int

func newDisjoint(n int) disjoint {
	d := make(disjoint, n)
	for i := range d {
		d[i] = i
	}
	return d
}

func (d disjoint) find(i int) int {
	for d[i] != i {
		d[i] = d[d[i]]
		i = d[i]
	}
	return i
}

func (d disjoint) union(i, j int) { d[d.find(i)] = d.find(j) }

// wholeFrameEvents pulls out cuts and flashes before anything else, so one
// enormous transition does not swallow the events around it.
func (a *Analyzer) wholeFrameEvents(opt TimelineOptions, consumed []bool) []Event {
	s := a.samples
	var events []Event
	for _, sp := range runs(len(s), func(i int) bool { return s[i].Changed >= opt.CutFraction }, 1) {
		if sp.to-sp.from+1 > 2 {
			continue // a long whole-frame run is sustained activity, not a boundary
		}
		peak, sum := 0.0, 0.0
		for i := sp.from; i <= sp.to; i++ {
			consumed[i] = true
			peak = math.Max(peak, s[i].Changed)
			sum += s[i].Changed
		}
		whole := image.Rect(0, 0, a.width, a.height)
		persists := a.persists(whole, s[sp.from].Time, s[sp.to].Time)
		// One transition that stays is a boundary; anything that comes back is
		// a flash, however briefly it lasted.
		kind := KindCut
		if sp.to > sp.from || reverted(persists) {
			kind = KindFlash
		}
		e := Event{
			Kind: kind, Start: s[sp.from].Time, End: s[sp.to].Time, Peak: s[sp.from].Time,
			PeakChanged: peak, MeanChanged: sum / float64(sp.to-sp.from+1),
			Region: a.sourceRect(whole, opt), RegionArea: 1, Position: "whole frame",
			Persists: persists,
		}
		e.round()
		e.Summary = wholeFrameSummary(e)
		events = append(events, e)
	}
	return events
}

// runs groups indices satisfying match into spans, joining spans separated by
// at most gap non-matching samples.
func runs(n int, match func(int) bool, gap int) []span {
	var out []span
	current := span{-1, -1}
	miss := 0
	for i := 0; i < n; i++ {
		if match(i) {
			if current.from < 0 {
				current.from = i
			}
			current.to = i
			miss = 0
			continue
		}
		if current.from < 0 {
			continue
		}
		miss++
		if miss > gap {
			out = append(out, current)
			current = span{-1, -1}
			miss = 0
		}
	}
	if current.from >= 0 {
		out = append(out, current)
	}
	return out
}
