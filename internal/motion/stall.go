package motion

import (
	"fmt"
	"math"
)

// minStall is the shortest gap worth reporting as a freeze. Below about a
// second, a pause in an animation is ordinary.
const minStall = 0.8

// minSurrounding is how long the activity either side must run for its absence
// to mean anything. A gap between two instantaneous events is not a freeze.
const minSurrounding = 0.5

// stalls finds stretches where activity that was running continuously stopped
// and then resumed in the same place.
//
// This exists because a freeze is an absence of change, and nothing else in the
// tool can express it: no pixel moved, so no event describes it. Reported only
// as a "quiet range" it reads exactly like a screen that was meant to be still,
// which is the opposite of what it means. On a recording with a heartbeat it is
// usually the most important thing that happened.
func stalls(events []Event, samples []Sample) []Event {
	// Only continuously running events can stop. Brief ones are filtered out
	// first, or a blip landing inside a long event's span would break the
	// adjacency between that event and its own resumption.
	var running []Event
	for _, e := range events {
		if e.End-e.Start >= minSurrounding && e.Kind != KindStall {
			running = append(running, e)
		}
	}
	var out []Event
	for i := 0; i+1 < len(running); i++ {
		before, after := running[i], running[i+1]
		if after.Start-before.End < minStall {
			continue
		}
		if !overlapping(before.Region, after.Region) {
			continue // two different things, not one thing pausing
		}
		if interrupted(events, before.Region, before.End, after.Start) {
			continue // something did happen there; it did not stop
		}
		out = append(out, stallEvent(before, after, samples))
	}
	return out
}

// interrupted reports whether anything happened in the region during the gap.
func interrupted(events []Event, region [4]int, start, end float64) bool {
	for _, e := range events {
		if e.Start < end && start < e.End && overlapping(e.Region, region) {
			return true
		}
	}
	return false
}

func stallEvent(before, after Event, samples []Sample) Event {
	e := Event{
		Kind: KindStall, Start: before.End, End: after.Start, Peak: before.End,
		Region: before.Region, RegionArea: before.RegionArea, Position: before.Position,
	}
	peak, frozen := quietPeak(samples, e.Start, e.End)
	e.PeakChanged = peak
	e.round()

	duration := e.End - e.Start
	what := "nothing changed by more than the threshold"
	if frozen {
		what = "not a single pixel changed"
	}
	e.Summary = fmt.Sprintf(
		"The %s (%s) was changing continuously and then stopped for %.2fs, from %s to %s, before resuming. Across the whole frame %s during that time. An absence of change on a recording that is otherwise moving is what a freeze or a hang looks like.",
		e.Position, regionSize(e.Region), duration, clock(e.Start), clock(e.End), what)
	return e
}

// quietPeak reports the largest change seen during the gap, and whether the
// frame was literally identical throughout.
func quietPeak(samples []Sample, start, end float64) (float64, bool) {
	peak, frozen := 0.0, true
	for _, s := range samples {
		if s.Time <= start || s.Time >= end {
			continue
		}
		peak = math.Max(peak, s.Changed)
		if s.Changed > 0 {
			frozen = false
		}
	}
	return round4(peak), frozen
}

func overlapping(a, b [4]int) bool {
	return a[0] < b[2] && b[0] < a[2] && a[1] < b[3] && b[1] < a[3]
}
