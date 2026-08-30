package motion

import (
	"fmt"
	"math"
)

// Suitability verdicts.
const (
	FitGood     = "suitable"
	FitMarginal = "marginal"
	FitPoor     = "unsuitable"
)

// Assessment says how much this recording resembles the fixed-viewport footage
// the tool works on. Without it, continuously moving footage produces a long
// list of confident-sounding events that mean nothing, and a caller has no way
// to tell that from a real finding.
type Assessment struct {
	Verdict string `json:"verdict"`
	// GlobalMotion is the share of the analysed interval during which change
	// covers most of the frame at once. It is the measure that separates a
	// screen with one animating widget from footage where a pan, a zoom, or
	// ambient motion such as wind, water, fire or grain keeps everywhere
	// moving. Both are busy every frame; only one is busy everywhere.
	GlobalMotion float64 `json:"global_motion_share"`
	// TypicalChanged is the median share of the frame changing per transition.
	TypicalChanged float64 `json:"typical_changed_fraction"`
	Reason         string  `json:"reason"`
	Advice         string  `json:"advice,omitempty"`
}

// Thresholds separating a fixed viewport from footage that never holds still.
// They answer different questions and are deliberately not unified: one is how
// long the whole frame is in motion, the other how much of it moves at once.
const (
	poorGlobalMotion      = 0.5
	marginalGlobalMotion  = 0.2
	poorTypicalChange     = 0.25
	marginalTypicalChange = 0.06
)

// assess judges whether this recording is the kind the tool works on.
//
// The question is not "is anything moving" — a heartbeat makes that true of any
// dashboard — but "is everywhere moving, for most of the time". A pan, a slow
// zoom, and ambient motion such as wind, water, fire, a crowd or film grain all
// produce an event that covers most of the frame and lasts most of the
// interval. In that regime the smaller events are fragments of one moving scene
// and their boundaries are arbitrary.
func assess(events []Event, samples []Sample, span float64) Assessment {
	if len(samples) == 0 {
		return Assessment{Verdict: FitGood, Reason: "Nothing to measure."}
	}
	changed := make([]float64, len(samples))
	for i, s := range samples {
		changed[i] = s.Changed
	}
	typical := round4(median(changed))

	global := 0.0
	if span > 0 {
		for _, e := range events {
			// Cuts and flashes cover the whole frame by definition and last no
			// time at all; they are boundaries, not continuous motion.
			if e.RegionArea <= 0.6 || e.Kind == KindCut || e.Kind == KindFlash {
				continue
			}
			global = math.Max(global, (e.End-e.Start)/span)
		}
	}
	global = round4(global)

	switch {
	case global > poorGlobalMotion || typical > poorTypicalChange:
		return Assessment{
			Verdict: FitPoor, GlobalMotion: global, TypicalChanged: typical,
			Reason: fmt.Sprintf("Change covers most of the frame for %.0f%% of the interval. Something keeps the whole picture moving — a camera pan or zoom, a scroll, or ambient motion such as wind, water, fire, a crowd or film grain — rather than a fixed viewport with discrete changes.", global*100),
			Advice: "Do not read the events below as a list of findings: where everything moves, the small ones are fragments of one moving scene and their boundaries are arbitrary. Use 'sheet' and 'frames' to look at the content instead.",
		}
	case global > marginalGlobalMotion || typical > marginalTypicalChange:
		return Assessment{
			Verdict: FitMarginal, GlobalMotion: global, TypicalChanged: typical,
			Reason: fmt.Sprintf("Change covers most of the frame for %.0f%% of the interval, which is a lot for a fixed viewport.", global*100),
			Advice: "Some of the smaller events may be fragments of one continuously moving thing rather than separate findings. Look at a 'sheet' before relying on the event list.",
		}
	default:
		return Assessment{
			Verdict: FitGood, GlobalMotion: global, TypicalChanged: typical,
			Reason: "Most of the frame holds still, so a change in one place is a finding rather than part of a moving scene.",
		}
	}
}
