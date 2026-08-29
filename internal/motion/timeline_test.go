package motion_test

import (
	"context"
	"math"
	"testing"

	"github.com/shhac/agent-motion/internal/fixture"
	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/video"
)

// analyseReference runs the known scenario through the analyser at the given
// analysis width, with no decoder and no media on disk.
func analyseReference(t *testing.T, width, height int, opt motion.Options) (*motion.Analyzer, fixture.Scenario) {
	t.Helper()
	s := fixture.Reference()
	dec := &video.Fake{
		Info:   video.Info{Width: s.Width, Height: s.Height, FPS: s.FPS, Duration: s.Duration(), NBFrames: s.Frames},
		Render: s.Frame,
	}
	a := motion.New(width, height, opt)
	req := video.Request{Path: "reference", Width: width, Height: height, FPS: s.FPS}
	if err := dec.Decode(context.Background(), req, a.Add); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return a, s
}

func referenceTimeline(t *testing.T) (motion.Timeline, fixture.Scenario) {
	t.Helper()
	a, s := analyseReference(t, 320, 180, motion.Options{
		Threshold: 12, DriftFrames: 30, Checkpoints: 128, ExpectedFrames: s2frames(), IgnoreAbove: 0.5,
	})
	return a.Timeline(motion.TimelineOptions{
		FPS: s.FPS, SourceWidth: s.Width, SourceHeight: s.Height, DriftSeconds: 1, CutFraction: 0.5,
	}), s
}

func s2frames() int { return fixture.Reference().Frames }

// TestTimelineFindsEveryKnownEvent is the behavioural contract: the six things
// the reference scenario actually does must each be reported once, with the
// right kind, near the right time, over the right region.
func TestTimelineFindsEveryKnownEvent(t *testing.T) {
	timeline, scenario := referenceTimeline(t)

	want := []struct {
		event string
		kind  string
		start float64
	}{
		{"moving-dot", motion.KindMotion, 2},
		{"appear-badge", motion.KindStep, 6.5},
		{"flicker-panel", motion.KindFlicker, 9},
		{"scene-cut", motion.KindCut, 15},
		{"single-frame-flash", motion.KindFlash, 21},
		{"fade-region", motion.KindGradual, 23},
	}
	for _, w := range want {
		got := findNear(timeline.Events, w.start, 1.0)
		if got == nil {
			t.Errorf("%s: no event within 1s of %.2fs; got %v", w.event, w.start, kinds(timeline.Events))
			continue
		}
		if got.Kind != w.kind {
			t.Errorf("%s at %.2fs: kind = %q, want %q", w.event, got.Start, got.Kind, w.kind)
		}
		if truth := regionOf(scenario, w.event); !overlapsWell(got.Region, truth) {
			t.Errorf("%s: region %v does not match the known region %v", w.event, got.Region, truth)
		}
		if got.Summary == "" {
			t.Errorf("%s: empty summary", w.event)
		}
	}
	// The second scene cut is a separate event; nothing else should appear.
	if len(timeline.Events) != len(want)+1 {
		t.Errorf("got %d events %v, want %d (the six known events plus the second cut)",
			len(timeline.Events), kinds(timeline.Events), len(want)+1)
	}
}

func TestMotionEventReportsTravelDirection(t *testing.T) {
	timeline, _ := referenceTimeline(t)
	e := findNear(timeline.Events, 2, 1)
	if e == nil {
		t.Fatal("no movement event")
	}
	if e.Direction != "left to right" {
		t.Errorf("direction = %q, want left to right", e.Direction)
	}
	if e.TravelPixels < 400 || e.TravelPixels > 500 {
		t.Errorf("travel = %d px, want roughly the 460 px the square actually covers", e.TravelPixels)
	}
}

func TestPersistenceSeparatesAppearanceFromFlash(t *testing.T) {
	timeline, _ := referenceTimeline(t)
	badge := findNear(timeline.Events, 6.5, 0.5)
	flash := findNear(timeline.Events, 21, 0.5)
	if badge == nil || badge.Persists == nil || !*badge.Persists {
		t.Errorf("the badge appears and stays, so persists should be true; got %+v", badge)
	}
	if flash == nil || flash.Persists == nil || *flash.Persists {
		t.Errorf("the flash reverts, so persists should be false; got %+v", flash)
	}
}

func TestFlickerReportsRate(t *testing.T) {
	timeline, _ := referenceTimeline(t)
	e := findNear(timeline.Events, 9, 0.5)
	if e == nil {
		t.Fatal("no flicker event")
	}
	// The panel toggles every 3 frames at 30 fps: 10 changes per second.
	if math.Abs(e.ChangesPerSecond-10) > 1.5 {
		t.Errorf("changes per second = %.2f, want about 10", e.ChangesPerSecond)
	}
}

// TestGradualNeedsTheSlowTimescale pins why drift exists: the fade is invisible
// to frame-to-frame differencing alone.
func TestGradualNeedsTheSlowTimescale(t *testing.T) {
	a, s := analyseReference(t, 320, 180, motion.Options{Threshold: 12, Checkpoints: 64, ExpectedFrames: s2frames()})
	timeline := a.Timeline(motion.TimelineOptions{
		FPS: s.FPS, SourceWidth: s.Width, SourceHeight: s.Height, CutFraction: 0.5,
	})
	if e := findNear(timeline.Events, 24, 2); e != nil {
		t.Errorf("without drift the fade should be invisible, but got %+v", e)
	}
}

func TestNoEventsInAStaticInterval(t *testing.T) {
	a, s := analyseReference(t, 160, 90, motion.Options{
		Threshold: 12, DriftFrames: 15, Checkpoints: 32, ExpectedFrames: s2frames(),
	})
	timeline := a.Timeline(motion.TimelineOptions{
		FPS: s.FPS, SourceWidth: s.Width, SourceHeight: s.Height, DriftSeconds: 0.5, CutFraction: 0.5,
	})
	for _, e := range timeline.Events {
		if e.End < 2 || e.Start > 27.9 {
			t.Errorf("event %+v falls in a stretch where the scenario does nothing", e)
		}
	}
}

func TestOverviewNarratesAndSuggests(t *testing.T) {
	a, s := analyseReference(t, 320, 180, motion.Options{
		Threshold: 12, DriftFrames: 30, Checkpoints: 128, ExpectedFrames: s2frames(), IgnoreAbove: 0.5,
	})
	timeline := a.Timeline(motion.TimelineOptions{
		FPS: s.FPS, SourceWidth: s.Width, SourceHeight: s.Height, DriftSeconds: 1, CutFraction: 0.5,
	})
	o := a.Overview(timeline, 40)
	if o.Narrative == "" {
		t.Fatal("empty narrative")
	}
	if len(o.Inspect) == 0 {
		t.Error("no timestamps suggested for inspection")
	}
	if len(o.Sparkline) != 40 {
		t.Errorf("sparkline has %d cells, want one per bucket", len(o.Sparkline))
	}
	if len(o.Quiet) == 0 {
		t.Error("the scenario is static for seconds at a time, so quiet ranges should exist")
	}
}

func TestIgnoreAboveKeepsCutsOutOfTheImage(t *testing.T) {
	a, _ := analyseReference(t, 160, 90, motion.Options{
		Threshold: 12, Checkpoints: 32, ExpectedFrames: s2frames(), IgnoreAbove: 0.5,
	})
	if got := len(a.Ignored()); got != 4 {
		t.Errorf("ignored %d transitions, want the 2 cuts and the 2 edges of the flash", got)
	}
	if a.Coverage() >= 1 {
		t.Errorf("coverage = %.4f; excluding whole-frame transitions should stop it saturating", a.Coverage())
	}
}

func findNear(events []motion.Event, at, tolerance float64) *motion.Event {
	for i := range events {
		if math.Abs(events[i].Start-at) <= tolerance {
			return &events[i]
		}
	}
	return nil
}

func kinds(events []motion.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Kind
	}
	return out
}

func regionOf(s fixture.Scenario, name string) [4]int {
	for _, e := range s.Events {
		if e.Name == name {
			return [4]int{e.Region.Min.X, e.Region.Min.Y, e.Region.Max.X, e.Region.Max.Y}
		}
	}
	return [4]int{}
}

// overlapsWell accepts a reported region that covers most of the true region
// without being wildly larger, which is the useful standard for a bounding box
// derived from downscaled analysis.
func overlapsWell(got, want [4]int) bool {
	ix := math.Min(float64(got[2]), float64(want[2])) - math.Max(float64(got[0]), float64(want[0]))
	iy := math.Min(float64(got[3]), float64(want[3])) - math.Max(float64(got[1]), float64(want[1]))
	if ix <= 0 || iy <= 0 {
		return false
	}
	intersection := ix * iy
	truth := float64((want[2] - want[0]) * (want[3] - want[1]))
	reported := float64((got[2] - got[0]) * (got[3] - got[1]))
	return intersection/truth > 0.6 && reported < truth*3
}
