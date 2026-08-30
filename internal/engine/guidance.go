package engine

import (
	"fmt"
	"math"

	"github.com/shhac/agent-motion/internal/motion"
)

// nextSteps turns findings into commands the caller can run verbatim.
func nextSteps(path string, o motion.Overview, t motion.Timeline) []string {
	var steps []string
	if len(o.Inspect) > 0 {
		steps = append(steps, fmt.Sprintf("agent-motion sheet %s --at %s", quote(path), joinTimes(o.Inspect)))
	}
	if narrow := narrowest(t.Events); narrow != nil {
		steps = append(steps, fmt.Sprintf("agent-motion timeline %s --start %.2f --end %.2f --threshold 4",
			quote(path), math.Max(0, narrow.Start-0.5), narrow.End+0.5))
	}
	if len(t.Events) == 0 {
		steps = append(steps, fmt.Sprintf("agent-motion timeline %s --threshold 4  # nothing found at the current threshold", quote(path)))
	}
	return steps
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

func limits(analysisWidth, sourceWidth int, threshold float64, fit motion.Assessment) []string {
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
