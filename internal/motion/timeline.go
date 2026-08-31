package motion

import (
	"sort"
)

// Event kinds. They describe the shape of a change, never its meaning.
const (
	KindCut     = "cut"     // one transition replaces most of the frame and it stays replaced
	KindFlash   = "flash"   // most of the frame changes for one or two frames, then returns
	KindStep    = "step"    // brief localised change that is still there afterwards
	KindBlip    = "blip"    // brief localised change that reverts
	KindFlicker = "flicker" // the same area toggles repeatedly
	KindMotion  = "motion"  // activity whose centre travels across the frame
	KindGradual = "gradual" // too slow to see frame to frame, clear over the drift window
	KindBusy    = "busy"    // sustained activity with no clearer shape
	KindStall   = "stall"   // activity that was running continuously stopped, then resumed
	KindShift   = "shift"   // content moved to a new position rather than appearing or changing
)

// Event is one described occurrence in source-video coordinates and seconds.
type Event struct {
	Kind        string  `json:"kind"`
	Start       float64 `json:"start_seconds"`
	End         float64 `json:"end_seconds"`
	Peak        float64 `json:"peak_seconds"`
	PeakChanged float64 `json:"peak_changed_fraction"`
	MeanChanged float64 `json:"mean_changed_fraction"`
	// PeakDrift is the strongest difference across the slow window. It is the
	// only non-zero measure for changes too slow to see frame to frame.
	PeakDrift  float64 `json:"peak_drift_fraction,omitempty"`
	Region     [4]int  `json:"region_xyxy"`
	RegionArea float64 `json:"region_area_fraction"`
	Position   string  `json:"position"`
	// ChangesPerSecond is set for repeating events.
	ChangesPerSecond float64 `json:"changes_per_second,omitempty"`
	// Direction and TravelPixels are set when the active centre moves.
	Direction    string `json:"direction,omitempty"`
	TravelPixels int    `json:"travel_pixels,omitempty"`
	// JumpPixels and JumpSeconds mark a step against the direction of travel:
	// movement that went backwards once, which is usually the fault rather than
	// the movement around it.
	JumpPixels  int     `json:"jump_backwards_pixels,omitempty"`
	JumpSeconds float64 `json:"jump_backwards_seconds,omitempty"`
	// MovedBy is the displacement of the content, in source pixels, when the
	// change was a translation rather than an appearance. Positive Y is down.
	MovedBy []int `json:"moved_by_pixels,omitempty"`
	// ShiftScore is CLS-shaped: the share of the frame affected multiplied by
	// how far it moved as a share of the frame. It is not Chrome's Cumulative
	// Layout Shift, which is measured from the DOM over a session window.
	ShiftScore float64 `json:"layout_shift_score,omitempty"`
	// Uniform marks a whole-frame change where every pixel moved through the
	// same brightness map — an overlay or a dim — rather than the content
	// changing. The picture underneath is still the same picture.
	//
	// A theme switch is not one of these, though it looks like it should be:
	// it moves background and text in opposite directions, which no single map
	// fits, so it reads as content changing. That is the right answer.
	Uniform bool `json:"uniform_shade_change,omitempty"`
	// ShadeResidual is how far the frame strayed from that uniform map, in
	// luminance units. Small means an overlay; large means new content.
	// ShadeFit is the share of the frame that followed that brightness map. A
	// modal dims most of the frame and puts a dialog on the rest, so this says
	// more than an average distance can.
	ShadeFit float64 `json:"shade_fit,omitempty"`
	// ShadeScale is the brightness multiplier that best explains the change.
	// Around 0.5 on a uniform change means the picture was dimmed to half, the
	// signature of a modal backdrop.
	ShadeScale float64 `json:"shade_scale,omitempty"`
	// Persists reports whether the region still looks different afterwards.
	// It is nil when there was no checkpoint to compare against.
	// Continuous marks activity that runs for much of the interval, steadily,
	// in one small fixed place. That is the shape of something animating rather
	// than something going wrong, and without it six seconds of "sustained
	// activity" reads as unresolved jank when it is a marquee.
	Continuous bool   `json:"continuous,omitempty"`
	Persists   *bool  `json:"persists,omitempty"`
	Summary    string `json:"summary"`
}

// Timeline is the described result of one analysis pass.
type Timeline struct {
	Events     []Event    `json:"events"`
	NoiseFloor float64    `json:"noise_floor_fraction"`
	Truncated  int        `json:"events_omitted,omitempty"`
	Fit        Assessment `json:"suitability"`
}

// Timeline segments the series in space and time, then describes each segment.
//
// Segmentation is per grid cell rather than per frame because real recordings
// almost always have something animating continuously — a spinner, a cursor, a
// clock. A purely temporal pass merges that with everything else and reports
// one enormous event covering the whole video.
func (a *Analyzer) Timeline(opt TimelineOptions) Timeline {
	opt = opt.withDefaults()
	if len(a.samples) == 0 {
		return Timeline{Events: []Event{}}
	}
	from, to := a.Span()
	opt.Span = to - from

	consumed := make([]bool, len(a.samples))
	events := a.wholeFrameEvents(opt, consumed)

	floors := a.cellFloors(opt.MinFloor)
	gap := a.gapSamples(opt.MergeGap, opt.FPS)
	spans := a.fastSpans(floors, consumed, gap)
	spans = append(spans, a.slowSpans(floors, consumed, gap, opt)...)

	for _, g := range groupSpans(spans, a.grid, gap) {
		if e, ok := a.describe(g, floors, opt); ok {
			events = append(events, e)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Start < events[j].Start })
	events = append(events, stalls(events, a.samples)...)
	sort.Slice(events, func(i, j int) bool { return events[i].Start < events[j].Start })

	t := Timeline{
		NoiseFloor: round4(median(floors)),
		Events:     events,
		Fit:        assess(events, a.samples, opt.Span),
	}
	if len(events) > opt.MaxEvents {
		t.Events = trimToBudget(events, opt.MaxEvents)
		t.Truncated = len(events) - len(t.Events)
	}
	return t
}

// trimToBudget keeps the most prominent events but preserves time order.
//
// It ranks indices, not timestamps: two events can begin on the same transition
// in different parts of the frame, and keying on the start time would let both
// through one slot while events_omitted under-reported the difference.
func trimToBudget(events []Event, budget int) []Event {
	order := make([]int, len(events))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return prominence(events[order[i]]) > prominence(events[order[j]])
	})
	keep := make([]bool, len(events))
	for _, i := range order[:budget] {
		keep[i] = true
	}
	out := make([]Event, 0, budget)
	for i, e := range events {
		if keep[i] {
			out = append(out, e)
		}
	}
	return out
}
