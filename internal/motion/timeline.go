package motion

import (
	"math"
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
	// Persists reports whether the region still looks different afterwards.
	// It is nil when there was no checkpoint to compare against.
	Persists *bool  `json:"persists,omitempty"`
	Summary  string `json:"summary"`
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

	start, end := a.Span()
	t := Timeline{
		NoiseFloor: round4(median(floors)),
		Events:     events,
		Fit:        assess(events, a.samples, end-start),
	}
	if len(events) > opt.MaxEvents {
		t.Events = trimToBudget(events, opt.MaxEvents)
		t.Truncated = len(events) - len(t.Events)
	}
	return t
}

// trimToBudget keeps the most prominent events but preserves time order.
func trimToBudget(events []Event, budget int) []Event {
	ranked := append([]Event(nil), events...)
	sort.SliceStable(ranked, func(i, j int) bool {
		return math.Max(ranked[i].PeakChanged, ranked[i].PeakDrift) >
			math.Max(ranked[j].PeakChanged, ranked[j].PeakDrift)
	})
	keep := make(map[float64]bool, budget)
	for _, e := range ranked[:budget] {
		keep[e.Start] = true
	}
	out := make([]Event, 0, budget)
	for _, e := range events {
		if keep[e.Start] {
			out = append(out, e)
		}
	}
	return out
}
