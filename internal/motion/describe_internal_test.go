package motion

import (
	"image"
	"image/color"
	"testing"
)

// compass, classify and positionOf were only ever reached through a full
// fixture decode, so most of their branches had never run.
func TestCompassNamesEveryDirection(t *testing.T) {
	cases := []struct {
		dx, dy float64
		want   string
	}{
		{10, 0, "left to right"},
		{-10, 0, "right to left"},
		{0, 10, "top to bottom"},
		{0, -10, "bottom to top"},
		{10, 1, "left to right"}, // dominant axis wins at 2:1
		{-1, 10, "top to bottom"},
		{10, 10, "left to right and top to bottom"},
		{-10, 10, "right to left and top to bottom"},
		{10, -10, "left to right and bottom to top"},
		{-10, -10, "right to left and bottom to top"},
		{0, 0, ""},
	}
	for _, c := range cases {
		if got := compass(c.dx, c.dy); got != c.want {
			t.Errorf("compass(%v, %v) = %q, want %q", c.dx, c.dy, got, c.want)
		}
	}
}

func TestClassifyBranches(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		name  string
		event Event
		stats groupStats
		want  string
	}{
		{"one transition that persists", Event{Persists: &yes}, groupStats{active: 1, frames: 1}, KindStep},
		{"one transition that reverts", Event{Persists: &no}, groupStats{active: 1, frames: 1}, KindBlip},
		{"two transitions, persistence unknown", Event{}, groupStats{active: 2, frames: 8}, KindBlip},
		{"repeating in one place", Event{}, groupStats{active: 6, frames: 20}, KindFlicker},
		{"repeating but travelling", Event{TravelPixels: 40}, groupStats{active: 6, frames: 20}, KindMotion},
		{"active every frame, going nowhere", Event{}, groupStats{active: 20, frames: 20}, KindBusy},
		{"active every frame, travelling", Event{TravelPixels: 40}, groupStats{active: 20, frames: 20}, KindMotion},
	}
	for _, c := range cases {
		if got := classify(c.event, c.stats); got != c.want {
			t.Errorf("%s: classify = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPositionOfNamesEachThird(t *testing.T) {
	opt := TimelineOptions{SourceWidth: 900, SourceHeight: 900}
	cases := map[string][4]int{
		"top left":      {0, 0, 100, 100},
		"top centre":    {400, 0, 500, 100},
		"top right":     {800, 0, 900, 100},
		"middle left":   {0, 400, 100, 500},
		"middle centre": {400, 400, 500, 500},
		"bottom right":  {800, 800, 900, 900},
		"whole frame":   {0, 0, 900, 900},
	}
	for want, box := range cases {
		if got := positionOf(box, opt); got != want {
			t.Errorf("positionOf(%v) = %q, want %q", box, got, want)
		}
	}
	if got := positionOf([4]int{0, 0, 10, 10}, TimelineOptions{}); got != "frame" {
		t.Errorf("with no source size, positionOf = %q, want %q", got, "frame")
	}
}

// trimToBudget must honour the budget exactly, including when events share a
// start time — which per-cell segmentation produces routinely.
func TestTrimToBudgetHonoursTheBudget(t *testing.T) {
	events := []Event{
		{Start: 1, PeakChanged: 0.1},
		{Start: 1, PeakChanged: 0.9},
		{Start: 2, PeakChanged: 0.5},
		{Start: 3, PeakChanged: 0.2},
	}
	for budget := 1; budget <= len(events); budget++ {
		got := trimToBudget(events, budget)
		if len(got) != budget {
			t.Errorf("budget %d kept %d events", budget, len(got))
		}
		for i := 1; i < len(got); i++ {
			if got[i].Start < got[i-1].Start {
				t.Errorf("budget %d returned events out of time order: %v", budget, got)
			}
		}
	}
	if kept := trimToBudget(events, 1); kept[0].PeakChanged != 0.9 {
		t.Errorf("the budget should keep the most prominent event, kept %+v", kept[0])
	}
}

func TestCompareCountsAndBoundsTheDifference(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			base.Set(x, y, color.Black)
		}
	}
	same := *base
	if d := Compare(base, &same, 12); !d.Identical() || d.Changed != 0 {
		t.Errorf("identical images should compare identical, got %+v", d)
	}

	changed := image.NewRGBA(base.Bounds())
	copy(changed.Pix, base.Pix)
	for y := 3; y < 6; y++ {
		for x := 2; x < 4; x++ {
			changed.Set(x, y, color.White)
		}
	}
	d := Compare(base, changed, 12)
	if d.Changed != 6 {
		t.Errorf("changed = %d, want the 6 pixels actually altered", d.Changed)
	}
	if want := image.Rect(2, 3, 4, 6); d.Box != want {
		t.Errorf("box = %v, want %v", d.Box, want)
	}
	if d.Identical() || d.MaxDelta != 255 {
		t.Errorf("got %+v, want a maximum delta of 255", d)
	}
	if got, want := d.Fraction(), 0.06; got < want-0.001 || got > want+0.001 {
		t.Errorf("fraction = %v, want %v", got, want)
	}
}

// A difference below the threshold is not the same as no difference, and the
// distinction is the whole point of comparing two moments.
func TestCompareSeparatesSubThresholdFromIdentical(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 4, 4))
	b := image.NewRGBA(a.Bounds())
	for i := range a.Pix {
		a.Pix[i] = 100
		b.Pix[i] = 100
	}
	b.Set(1, 1, color.RGBA{104, 104, 104, 255})

	d := Compare(a, b, 12)
	if d.Changed != 0 {
		t.Errorf("a 4/255 difference should not clear a threshold of 12, got %d", d.Changed)
	}
	if d.Identical() {
		t.Error("frames that differ at all must not be reported as identical")
	}
}
