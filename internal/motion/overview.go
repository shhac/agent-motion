package motion

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Overview is the orientation layer: what the whole interval looks like before
// an agent reads individual events.
type Overview struct {
	Narrative     string       `json:"narrative"`
	Busiest       float64      `json:"busiest_seconds"`
	Quiet         [][2]float64 `json:"quiet_ranges,omitempty"`
	BucketSeconds float64      `json:"bucket_seconds,omitempty"`
	Sparkline     string       `json:"activity_sparkline,omitempty"`
	PeakBucket    float64      `json:"activity_sparkline_full_scale,omitempty"`
	Activity      []float64    `json:"activity_by_bucket,omitempty"`
	Inspect       []float64    `json:"timestamps_worth_inspecting,omitempty"`
}

// minQuiet keeps the quiet list to intervals long enough to be worth skipping.
const minQuiet = 0.5

// Overview summarises a timeline over the analysed span.
func (a *Analyzer) Overview(t Timeline, buckets int) Overview {
	start, end := a.Span()
	o := Overview{
		Quiet:    quietRanges(t.Events, start, end),
		Busiest:  busiest(a.samples),
		Inspect:  inspectTimes(t.Events),
		Activity: bucketActivity(a.samples, start, end, buckets),
	}
	if len(o.Activity) > 0 {
		o.BucketSeconds = round2((end - start) / float64(len(o.Activity)))
		o.Sparkline, o.PeakBucket = sparkline(o.Activity)
	}
	o.Narrative = narrate(t, o, start, end)
	return o
}

func narrate(t Timeline, o Overview, start, end float64) string {
	var b strings.Builder
	// An unsuitable recording is the first thing a reader needs to know, before
	// any finding that might be read as meaningful.
	if t.Fit.Verdict != FitGood {
		b.WriteString(t.Fit.Reason + " " + t.Fit.Advice + " ")
	}
	fmt.Fprintf(&b, "Analysed %s to %s. ", clock(start), clock(end))
	if len(t.Events) == 0 {
		b.WriteString("Nothing above the noise floor changed anywhere in this interval.")
		return b.String()
	}
	fmt.Fprintf(&b, "Found %s. ", countKinds(t.Events))
	// A stall is the one finding that is an absence, so it has to be said
	// rather than left for a reader to notice in the quiet ranges.
	for _, e := range t.Events {
		if e.Kind == KindStall {
			fmt.Fprintf(&b, "Note the stall: activity in the %s stopped from %s to %s (%.2fs) and then resumed, which is what a freeze looks like. ",
				e.Position, clock(e.Start), clock(e.End), e.End-e.Start)
		}
	}
	fmt.Fprintf(&b, "The busiest moment is %s. ", clock(o.Busiest))
	switch {
	case len(o.Quiet) == 0:
		b.WriteString("Something is changing throughout.")
	case len(o.Quiet) <= 4:
		b.WriteString("Nothing changes during " + joinRanges(o.Quiet) + ".")
	default:
		fmt.Fprintf(&b, "There are %d quiet stretches, the longest being %s.",
			len(o.Quiet), rangeText(longestRange(o.Quiet)))
	}
	if t.Truncated > 0 {
		fmt.Fprintf(&b, " %d smaller events were omitted; narrow the interval to see them.", t.Truncated)
	}
	return b.String()
}

var kindNouns = map[string][2]string{
	KindCut:     {"hard cut", "hard cuts"},
	KindFlash:   {"whole-frame flash", "whole-frame flashes"},
	KindStep:    {"one-off change that persists", "one-off changes that persist"},
	KindBlip:    {"brief reverting change", "brief reverting changes"},
	KindFlicker: {"repeated toggle", "repeated toggles"},
	KindMotion:  {"movement", "movements"},
	KindGradual: {"gradual change", "gradual changes"},
	KindBusy:    {"stretch of sustained activity", "stretches of sustained activity"},
	KindStall:   {"stall where continuous activity stopped", "stalls where continuous activity stopped"},
}

func countKinds(events []Event) string {
	counts := map[string]int{}
	var order []string
	for _, e := range events {
		if counts[e.Kind] == 0 {
			order = append(order, e.Kind)
		}
		counts[e.Kind]++
	}
	sort.Slice(order, func(i, j int) bool { return counts[order[i]] > counts[order[j]] })
	parts := make([]string, 0, len(order))
	for _, k := range order {
		nouns, ok := kindNouns[k]
		if !ok {
			nouns = [2]string{k, k}
		}
		noun := nouns[0]
		if counts[k] > 1 {
			noun = nouns[1]
		}
		parts = append(parts, fmt.Sprintf("%d %s", counts[k], noun))
	}
	return joinList(parts)
}

func joinList(parts []string) string {
	switch len(parts) {
	case 0:
		return "nothing"
	case 1:
		return parts[0]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
}

func quietRanges(events []Event, start, end float64) [][2]float64 {
	busy := make([][2]float64, 0, len(events))
	for _, e := range events {
		if e.Kind == KindStall {
			continue // a stall is a quiet stretch; it should still read as one
		}
		busy = append(busy, [2]float64{e.Start, math.Max(e.End, e.Start)})
	}
	sort.Slice(busy, func(i, j int) bool { return busy[i][0] < busy[j][0] })

	var out [][2]float64
	cursor := start
	for _, b := range busy {
		if b[0]-cursor >= minQuiet {
			out = append(out, [2]float64{round2(cursor), round2(b[0])})
		}
		cursor = math.Max(cursor, b[1])
	}
	if end-cursor >= minQuiet {
		out = append(out, [2]float64{round2(cursor), round2(end)})
	}
	return out
}

func busiest(s []Sample) float64 {
	best, at := -1.0, 0.0
	for _, x := range s {
		if x.Changed > best {
			best, at = x.Changed, x.Time
		}
	}
	return round2(at)
}

// inspectTimes proposes frame timestamps that would show each event, capped so
// the list stays cheap to act on.
func inspectTimes(events []Event) []float64 {
	const budget = 12
	ranked := append([]Event(nil), events...)
	sort.SliceStable(ranked, func(i, j int) bool {
		return prominence(ranked[i]) > prominence(ranked[j])
	})
	seen := map[float64]bool{}
	var out []float64
	for _, e := range ranked {
		at := round2(e.Peak)
		if e.Kind == KindGradual {
			at = round2((e.Start + e.End) / 2)
		}
		if seen[at] {
			continue
		}
		seen[at] = true
		out = append(out, at)
		if len(out) == budget {
			break
		}
	}
	sort.Float64s(out)
	return out
}

func bucketActivity(s []Sample, start, end float64, buckets int) []float64 {
	if buckets <= 0 || len(s) == 0 || end <= start {
		return nil
	}
	buckets = min(buckets, len(s))
	out := make([]float64, buckets)
	width := (end - start) / float64(buckets)
	for _, x := range s {
		i := min(buckets-1, int((x.Time-start)/width))
		if i < 0 {
			i = 0
		}
		// Frame-to-frame change only. Including drift saturates the line on any
		// continuous texture — shimmering foliage, film grain — and a summary
		// that is always full is not a summary.
		out[i] = math.Max(out[i], x.Changed)
	}
	for i := range out {
		out[i] = round3(out[i])
	}
	return out
}

func joinRanges(r [][2]float64) string {
	parts := make([]string, 0, len(r))
	for _, x := range r {
		parts = append(parts, rangeText(x))
	}
	return joinList(parts)
}

func rangeText(r [2]float64) string { return fmt.Sprintf("%s-%s", clock(r[0]), clock(r[1])) }

func longestRange(r [][2]float64) [2]float64 {
	best := r[0]
	for _, x := range r {
		if x[1]-x[0] > best[1]-best[0] {
			best = x
		}
	}
	return best
}

// sparkline draws the activity series as one short string. The ramp is square
// root scaled against the peak bucket, because a single whole-frame change
// would otherwise flatten every ordinary event to nothing.
func sparkline(values []float64) (string, float64) {
	peak := 0.0
	for _, v := range values {
		peak = math.Max(peak, v)
	}
	if peak <= 0 {
		return strings.Repeat("_", len(values)), 0
	}
	const ramp = "_.:-=+*#"
	out := make([]byte, len(values))
	for i, v := range values {
		level := int(math.Round(math.Sqrt(v/peak) * float64(len(ramp)-1)))
		out[i] = ramp[min(len(ramp)-1, max(0, level))]
	}
	return string(out), round3(peak)
}
