package motion_test

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/shhac/agent-motion/internal/fixture"
	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/video"
)

// The defect scenario is the realistic case: a page with a heartbeat animating
// throughout, and three faults that a purely temporal segmentation would merge
// into one useless "something is happening" event.
func defectTimeline(t *testing.T) (motion.Timeline, motion.Overview) {
	t.Helper()
	s := fixture.Defect()
	dec := s.Decoder()
	a := motion.New(320, 180, motion.Options{
		Threshold: 12, DriftFrames: 30, Checkpoints: 128, ExpectedFrames: s.Frames, IgnoreAbove: 0.5,
	})
	err := dec.Decode(context.Background(), video.Request{Path: "x", Width: 320, Height: 180, FPS: s.FPS}, a.Add)
	if err != nil {
		t.Fatal(err)
	}
	timeline := a.Timeline(motion.TimelineOptions{
		FPS: s.FPS, SourceWidth: s.Width, SourceHeight: s.Height, DriftSeconds: 1, CutFraction: 0.5,
	})
	return timeline, a.Overview(timeline, 60)
}

// TestHeartbeatStaysInItsOwnCorner is the regression guard for the failure that
// forced spatial segmentation: one continuously animating element reported as a
// single event spanning most of the frame, hiding everything else.
func TestHeartbeatStaysInItsOwnCorner(t *testing.T) {
	timeline, _ := defectTimeline(t)
	for _, e := range timeline.Events {
		if e.Kind != motion.KindFlicker {
			continue
		}
		if e.RegionArea > 0.02 {
			t.Errorf("the heartbeat is a 20x20 dot but was reported over %.1f%% of the frame: %+v",
				e.RegionArea*100, e)
		}
		if e.Region[0] < 500 || e.Region[1] > 60 {
			t.Errorf("heartbeat region %v is not the top-right dot", e.Region)
		}
	}
}

func TestJitterIsFoundBesideTheHeartbeat(t *testing.T) {
	timeline, _ := defectTimeline(t)
	found := false
	for _, e := range timeline.Events {
		if e.Kind == motion.KindBlip && math.Abs(e.Start-6.2) < 0.3 {
			found = true
			// A 2px shift only changes the card's vertical edges.
			if e.Region[2]-e.Region[0] > 20 {
				t.Errorf("jitter region %v is wider than the edge that actually moved", e.Region)
			}
		}
	}
	if !found {
		t.Errorf("the 5-frame card shift at 6.2s was not reported; got %v", kinds(timeline.Events))
	}
}

// A stall shows up as an absence of change, which the events cannot express.
// It has to reach the caller through the quiet ranges and the narrative.
func TestStallIsReportedAsQuiet(t *testing.T) {
	_, overview := defectTimeline(t)
	found := false
	for _, r := range overview.Quiet {
		if r[0] > 10.5 && r[0] < 11.5 && r[1] > 13.5 && r[1] < 14.5 {
			found = true
		}
	}
	if !found {
		t.Errorf("the 11-14s stall is not in the quiet ranges: %v", overview.Quiet)
	}
	if !strings.Contains(overview.Narrative, "10.83s-14.00s") {
		t.Errorf("the narrative should name the stall: %q", overview.Narrative)
	}
}

func TestSlowDriftSurvivesAConstantlyAnimatingNeighbour(t *testing.T) {
	timeline, _ := defectTimeline(t)
	for _, e := range timeline.Events {
		if e.Kind != motion.KindGradual {
			continue
		}
		if e.Region[0] < 400 || e.Region[1] < 200 {
			t.Errorf("gradual region %v is not the status block at 450,250", e.Region)
		}
		return
	}
	t.Errorf("the status colour drift was not found; got %v", kinds(timeline.Events))
}
