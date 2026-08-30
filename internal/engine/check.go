package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/shhac/agent-motion/internal/motion"
)

// Assertion is one condition a recording must satisfy.
type Assertion struct {
	Name    string  `json:"name"`
	Passed  bool    `json:"passed"`
	Limit   float64 `json:"limit,omitempty"`
	Worst   float64 `json:"worst,omitempty"`
	Detail  string  `json:"detail"`
	Culprit string  `json:"culprit,omitempty"`
}

// CheckResult is a pass/fail verdict with the reasoning kept.
type CheckResult struct {
	Input       string            `json:"input"`
	Passed      bool              `json:"passed"`
	Assertions  []Assertion       `json:"assertions"`
	Narrative   string            `json:"narrative"`
	Suitability motion.Assessment `json:"suitability"`
	Events      []motion.Event    `json:"events"`
	// Notes accumulate. A verdict can be both unasserted and unjudgeable, and
	// dropping either for the other is how a green build comes to mean nothing.
	Notes []string `json:"notes,omitempty"`
}

// CheckOptions are the conditions to assert. Every threshold is opt-in: a check
// with none set asserts nothing and passes, which is the honest default.
type CheckOptions struct {
	Analyse AnalyseOptions

	MaxShiftScore  float64
	MaxShiftPixels int
	NoShift        bool
	NoStall        bool
	NoFlicker      bool
	Quiet          bool

	set map[string]bool
}

// Set records which thresholds the caller actually asked for, so an unset zero
// is not mistaken for a demand that nothing move at all.
func (o *CheckOptions) Set(name string) {
	if o.set == nil {
		o.set = map[string]bool{}
	}
	o.set[name] = true
}

func (o CheckOptions) asked(name string) bool { return o.set[name] }

// Check analyses a recording and asserts conditions against it, so a visual
// regression can fail a build rather than waiting to be noticed.
//
// Every assertion carries the event that broke it. A check that says only
// "failed" leaves whoever reads the CI log exactly where they started.
func (e *Engine) Check(ctx context.Context, opt CheckOptions) (*CheckResult, error) {
	analysis, err := e.Analyse(ctx, opt.Analyse)
	if err != nil {
		return nil, err
	}
	result := &CheckResult{
		Input: opt.Analyse.Path, Passed: true,
		Narrative: analysis.Narrative, Suitability: analysis.Suitability,
		Events: analysis.Events,
	}
	if analysis.Suitability.Verdict == motion.FitPoor {
		result.Notes = append(result.Notes, "This recording is not the kind the tool can judge — most of the frame is moving throughout, so these assertions are close to meaningless. Fix the capture before trusting a pass.")
	}

	if opt.asked("max-shift-score") {
		result.add(worstShift(analysis.Events, opt.MaxShiftScore,
			"max-shift-score", func(e motion.Event) float64 { return e.ShiftScore },
			"layout shift score"))
	}
	if opt.asked("max-shift-pixels") {
		result.add(worstShift(analysis.Events, float64(opt.MaxShiftPixels),
			"max-shift-pixels", func(e motion.Event) float64 {
				return float64(max(abs(e.MovedBy[0]), abs(e.MovedBy[1])))
			}, "pixels moved"))
	}
	if opt.NoShift {
		result.add(absent(analysis.Events, motion.KindShift, "no-shift", "content moved"))
	}
	if opt.NoStall {
		result.add(absent(analysis.Events, motion.KindStall, "no-stall", "activity stopped and resumed"))
	}
	if opt.NoFlicker {
		result.add(absent(analysis.Events, motion.KindFlicker, "no-flicker", "something toggled repeatedly"))
	}
	if opt.Quiet {
		a := Assertion{Name: "quiet", Passed: len(analysis.Events) == 0,
			Detail: "nothing at all changes above the threshold"}
		if !a.Passed {
			a.Detail = fmt.Sprintf("%d change(s) found; expected none", len(analysis.Events))
			a.Culprit = analysis.Events[0].Summary
		}
		result.add(a)
	}
	if len(result.Assertions) == 0 {
		result.Notes = append(result.Notes, "No conditions were given, so nothing was asserted. Pass --no-shift, --max-shift-score, --no-stall, --no-flicker or --quiet.")
	}
	return result, nil
}

func (r *CheckResult) add(a Assertion) {
	r.Assertions = append(r.Assertions, a)
	if !a.Passed {
		r.Passed = false
	}
}

// WorstShiftFor is exported for tests: it asserts that no one-off shift exceeds
// a limit.
func WorstShiftFor(events []motion.Event, limit float64) Assertion {
	return worstShift(events, limit, "max-shift-score",
		func(e motion.Event) float64 { return e.ShiftScore }, "layout shift score")
}

// worstShift asserts that no shift exceeds a limit, and names the worst one.
func worstShift(events []motion.Event, limit float64, name string, measure func(motion.Event) float64, unit string) Assertion {
	a := Assertion{Name: name, Passed: true, Limit: round4(limit)}
	var worst *motion.Event
	ignored := 0
	for i := range events {
		if events[i].Kind != motion.KindShift {
			continue
		}
		// A ticker step is a real translation and not what a layout-shift gate
		// is for; counting it would fail every page with a marquee.
		if events[i].Continuous {
			ignored++
			continue
		}
		if value := measure(events[i]); value > a.Worst {
			a.Worst, worst = round4(value), &events[i]
		}
	}
	if worst == nil {
		a.Detail = fmt.Sprintf("no layout shift found, so %s stayed at 0 (limit %g)%s", unit, limit, excluded(ignored))
		return a
	}
	a.Culprit = worst.Summary
	if a.Worst > limit {
		a.Passed = false
		a.Detail = fmt.Sprintf("worst %s is %g at %.2fs, over the limit of %g", unit, a.Worst, worst.Start, limit)
		return a
	}
	a.Detail = fmt.Sprintf("worst %s is %g at %.2fs, within the limit of %g", unit, a.Worst, worst.Start, limit)
	return a
}

// absent asserts that no event of a kind occurred.
func absent(events []motion.Event, kind, name, what string) Assertion {
	a := Assertion{Name: name, Passed: true}
	var found []string
	ignored := 0
	for _, e := range events {
		if kind == motion.KindShift && e.Kind == kind && e.Continuous {
			ignored++
			continue
		}
		if e.Kind == kind {
			found = append(found, fmt.Sprintf("%.2fs", e.Start))
			if a.Culprit == "" {
				a.Culprit = e.Summary
			}
		}
	}
	if len(found) > 0 {
		a.Passed = false
		a.Detail = fmt.Sprintf("%s at %s", what, strings.Join(found, ", "))
		return a
	}
	a.Detail = "none found: " + what + " nowhere in the recording" + excluded(ignored)
	return a
}

// excluded names the movement that was deliberately not counted, so a pass is
// never quieter than it should be about what it chose to ignore.
func excluded(n int) string {
	if n == 0 {
		return ""
	}
	if n == 1 {
		return " (1 step of ongoing movement was not counted; a marquee is not a layout shift)"
	}
	return fmt.Sprintf(" (%d steps of ongoing movement were not counted; a marquee is not a layout shift)", n)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
