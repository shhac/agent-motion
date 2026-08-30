package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/shhac/agent-motion/internal/engine"
	"github.com/shhac/agent-motion/internal/fixture"
)

func layoutEngine() *engine.Engine { return engine.New(fixture.Layout().Decoder()) }

func check(t *testing.T, mutate func(*engine.CheckOptions)) *engine.CheckResult {
	t.Helper()
	opt := engine.CheckOptions{Analyse: engine.Defaults("layout.mp4")}
	mutate(&opt)
	result, err := layoutEngine().Check(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// A check with nothing asked of it must pass and say why, rather than quietly
// implying the recording was examined and approved.
func TestCheckWithNoConditionsAssertsNothing(t *testing.T) {
	result := check(t, func(o *engine.CheckOptions) {})
	if !result.Passed || len(result.Assertions) != 0 {
		t.Errorf("got %+v, want a pass with no assertions", result)
	}
	if !strings.Contains(strings.Join(result.Notes, " "), "nothing was asserted") {
		t.Errorf("notes = %q, want one saying no conditions were given", result.Notes)
	}
}

func TestCheckFailsOnAShiftOverTheLimit(t *testing.T) {
	result := check(t, func(o *engine.CheckOptions) {
		o.MaxShiftScore = 0.001
		o.Set("max-shift-score")
	})
	if result.Passed {
		t.Fatal("a 40px shift should fail a score limit of 0.001")
	}
	a := result.Assertions[0]
	if a.Worst <= a.Limit {
		t.Errorf("worst %v is not above the limit %v, yet the check failed", a.Worst, a.Limit)
	}
	if a.Culprit == "" {
		t.Error("a failure must name the event that broke it, or a CI log leaves you where you started")
	}
}

func TestCheckPassesWhenTheLimitIsGenerous(t *testing.T) {
	result := check(t, func(o *engine.CheckOptions) {
		o.MaxShiftScore = 0.9
		o.Set("max-shift-score")
	})
	if !result.Passed {
		t.Errorf("a 0.9 limit should pass: %+v", result.Assertions)
	}
}

// An unset zero must not be read as "nothing may move at all" — that would make
// every check the strictest possible one by accident.
func TestUnsetThresholdIsNotAZeroThreshold(t *testing.T) {
	result := check(t, func(o *engine.CheckOptions) { o.MaxShiftScore = 0 })
	if len(result.Assertions) != 0 {
		t.Errorf("an unset threshold asserted %+v", result.Assertions)
	}
}

func TestNoShiftFailsAndNamesEveryOffender(t *testing.T) {
	result := check(t, func(o *engine.CheckOptions) { o.NoShift = true })
	if result.Passed {
		t.Fatal("the layout scenario shifts twice; --no-shift must fail")
	}
	if !strings.Contains(result.Assertions[0].Detail, "2.00s") ||
		!strings.Contains(result.Assertions[0].Detail, "4.00s") {
		t.Errorf("detail %q should list both shifts", result.Assertions[0].Detail)
	}
}

func TestConditionsThatDoNotApplyStillPass(t *testing.T) {
	result := check(t, func(o *engine.CheckOptions) {
		o.NoStall, o.NoFlicker = true, true
	})
	if !result.Passed {
		t.Errorf("the layout scenario has no stall or flicker: %+v", result.Assertions)
	}
}

func TestQuietFailsOnAnyChange(t *testing.T) {
	result := check(t, func(o *engine.CheckOptions) { o.Quiet = true })
	if result.Passed {
		t.Error("--quiet must fail on a recording where things happen")
	}
}

// A verdict on footage the tool cannot judge has to say so, or a green build
// means nothing.
func TestCheckWarnsWhenTheRecordingCannotBeJudged(t *testing.T) {
	dec := fixture.Reference().Decoder()
	dec.Render = panningNoise(dec.Info.Width, dec.Info.Height)
	result, err := engine.New(dec).Check(context.Background(), engine.CheckOptions{
		Analyse: engine.Defaults("pan.mp4"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(result.Notes, " "), "not the kind the tool can judge") {
		t.Errorf("notes = %q, want a warning that the assertions are meaningless here", result.Notes)
	}
}
