package engine

import (
	"fmt"
	"math"

	"github.com/shhac/agent-motion/internal/motion"
)

// nextSteps turns findings into commands the caller can run verbatim.
func nextSteps(opt AnalyseOptions, o motion.Overview, t motion.Timeline) []string {
	path := quote(opt.Path)
	var steps []string
	if len(o.Inspect) > 0 {
		steps = append(steps, fmt.Sprintf("agent-motion sheet %s --at %s", path, joinTimes(o.Inspect)))
	}
	if small := smallest(t.Events); small != nil {
		// A region a few pixels across is invisible in a full-frame still, so
		// point at a crop rather than leaving the reader to find one.
		steps = append(steps, fmt.Sprintf("agent-motion frames %s --at %.2f --region %d,%d,%d,%d --pad 24 --width 480",
			path, small.Peak, small.Region[0], small.Region[1], small.Region[2], small.Region[3]))
	}
	if sweep := cadenceSweep(path, t.Events); sweep != "" {
		steps = append(steps, sweep)
	}
	if narrow := narrowest(t.Events); narrow != nil {
		start, end := math.Max(0, narrow.Start-0.5), narrow.End+0.5
		threshold := opt.Threshold / 3
		// Proposing the interval and threshold that just ran is a no-op loop.
		if !sameRun(opt, start, end, threshold) {
			steps = append(steps, fmt.Sprintf("agent-motion timeline %s --start %.2f --end %.2f --threshold %.0f",
				path, start, end, math.Max(1, threshold)))
		}
	}
	if len(t.Events) == 0 && opt.Threshold > 4 {
		steps = append(steps, fmt.Sprintf("agent-motion timeline %s --threshold 4  # nothing cleared a threshold of %.0f", path, opt.Threshold))
	}
	if len(t.Events) == 0 && !opt.Native {
		steps = append(steps, fmt.Sprintf("agent-motion timeline %s --native  # look at full resolution", path))
	}
	return steps
}

// sameRun reports whether a proposed narrowing is what the caller already did.
func sameRun(opt AnalyseOptions, start, end, threshold float64) bool {
	return math.Abs(opt.Start-start) < 0.01 &&
		math.Abs(opt.End-end) < 0.01 &&
		math.Abs(opt.Threshold-threshold) < 0.01
}

// cadenceSweep proposes timestamps that show an event's internal shape rather
// than only its boundaries. An event's start and end say a panel toggled ten
// times a second or a colour drifted for four seconds; neither says what the
// toggle or the drift looks like, and working out the sampling by hand was a
// second round trip on every such event.
func cadenceSweep(path string, events []motion.Event) string {
	e := cadenced(events)
	if e == nil {
		return ""
	}
	count := 6
	if e.Kind == motion.KindFlicker && e.ChangesPerSecond > 0 {
		// Sample about twice per change, so consecutive tiles land on
		// alternating states rather than aliasing onto one of them.
		count = min(12, max(4, int((e.End-e.Start)*e.ChangesPerSecond*2)))
	}
	if e.End <= e.Start {
		return ""
	}
	return fmt.Sprintf("agent-motion sheet %s --during %.2f:%.2f --count %d --region %d,%d,%d,%d --pad 12 --width 240 --quick  # watch the %s",
		path, e.Start, e.End, count, e.Region[0], e.Region[1], e.Region[2], e.Region[3], e.Kind)
}

// cadenced picks the event whose internal shape is worth sampling.
func cadenced(events []motion.Event) *motion.Event {
	var best *motion.Event
	for i := range events {
		if events[i].Kind != motion.KindFlicker && events[i].Kind != motion.KindGradual {
			continue
		}
		if best == nil || prominence(events[i]) > prominence(*best) {
			best = &events[i]
		}
	}
	return best
}

func prominence(e motion.Event) float64 { return math.Max(e.PeakChanged, e.PeakDrift) }

// smallest picks the event hardest to see at full-frame scale.
func smallest(events []motion.Event) *motion.Event {
	var best *motion.Event
	for i := range events {
		if events[i].RegionArea == 0 || events[i].RegionArea > 0.05 {
			continue
		}
		if best == nil || events[i].RegionArea < best.RegionArea {
			best = &events[i]
		}
	}
	return best
}

// narrowest picks the event most likely to reward a closer look: the shortest
// one, because that is where a wide interval loses the most detail.
func narrowest(events []motion.Event) *motion.Event {
	var best *motion.Event
	for i := range events {
		if events[i].Kind == motion.KindCut {
			continue
		}
		if best == nil || events[i].End-events[i].Start < best.End-best.Start {
			best = &events[i]
		}
	}
	return best
}

func limits(p Params, sourceWidth int, sourceFPS float64, fit motion.Assessment) []string {
	analysisWidth, threshold := p.Width, p.Threshold
	out := []string{}
	if fit.Verdict != motion.FitGood {
		out = append(out, fit.Reason+" "+fit.Advice)
	}
	out = append(out,
		"Detects where and when pixels change. It does not identify objects, read text, or explain why something changed.",
		fmt.Sprintf("Anything changing a pixel by less than %.0f/255 per step is invisible; lower --threshold to see subtler change.", threshold),
	)
	if analysisWidth < sourceWidth {
		out = append(out, fmt.Sprintf("Analysed at %dpx wide, downscaled from %dpx; features thinner than about %d source pixels may be missed. Use --native for full resolution.",
			analysisWidth, sourceWidth, int(math.Ceil(float64(sourceWidth)/float64(analysisWidth)))))
	}
	if p.SampleFPS > 0 {
		out = append(out, fmt.Sprintf(
			"Timestamps are frame-scale, not exact: at %.3g fps every one is accurate to about %.0f ms, and seeking snaps to the nearest frame. Do not read them to more precision than that.",
			p.SampleFPS, 1000/p.SampleFPS))
	}
	if p.SampleFPS < sourceFPS-0.01 {
		out = append(out, fmt.Sprintf(
			"Sampled at %.3g fps from a %.3g fps source, so timestamps are coarser than the source allows and anything repeating faster than %.3g Hz is aliased — a flicker can be reported as a one-off change, or missed. Drop --sample-fps to see every frame.",
			p.SampleFPS, sourceFPS, p.SampleFPS/2))
	}
	if p.DriftSeconds <= 0 {
		out = append(out, "The slow timescale is off, so change too gradual to clear the threshold between adjacent frames — a fade, an easing animation, a creeping layout shift — is invisible. Raise --drift to see it.")
	}
	out = append(out, "A shift is found by registering one frame against the other, which needs most of the content to still be on screen afterwards. A move of more than about half the region it happened in — a page jumping a whole screen, a scroll — leaves too little overlap to measure, and is reported as a change rather than as a movement. 'No shift' is not 'nothing moved'.")
	out = append(out, "Event boundaries depend on --threshold and --analysis-width: a lower threshold can merge, split or re-scope events rather than only adding them, so two runs are not directly comparable.")
	return append(out, "Regions are bounding boxes of change, not object outlines. Look at the frames before drawing conclusions.")
}

func joinTimes(times []float64) string {
	out := ""
	for i, t := range times {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("%.2f", t)
	}
	return out
}

func quote(path string) string {
	for _, c := range path {
		if c == ' ' || c == '\'' || c == '"' {
			return fmt.Sprintf("%q", path)
		}
	}
	return path
}
