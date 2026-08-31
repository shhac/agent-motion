package engine_test

import (
	"context"
	"testing"

	"github.com/shhac/agent-motion/internal/engine"
	"github.com/shhac/agent-motion/internal/fixture"
)

func activityOf(t *testing.T, s fixture.Scenario) *engine.Analysis {
	t.Helper()
	opt := engine.Defaults(s.Name + ".mp4")
	opt.Cells = true
	a, err := engine.New(s.Decoder()).Analyse(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// The player fixture's ground truth says where each of its events is. Naming
// those places in text is the whole point of the command: an agent that cannot
// look at the activity image has no other way to ask where something happened.
func TestActivityLocatesFixtureEventsByPlace(t *testing.T) {
	s := fixture.Player()
	a := activityOf(t, s)
	if len(a.Cells) == 0 {
		t.Fatal("no cells reported for a recording with a bar moving throughout")
	}
	if a.Grid == "" {
		t.Error("cells are meaningless without the grid they came from")
	}

	// The bar sweeps the width of the frame, so no one cell holds it for long;
	// together the busy cells should cover most of the distance it travels.
	bar := s.Events[0].Region
	covered := 0
	for _, c := range a.Cells {
		if c.Box[1] < bar.Max.Y && c.Box[3] > bar.Min.Y {
			covered += min(c.Box[2], bar.Max.X) - max(c.Box[0], bar.Min.X)
		}
	}
	if covered < bar.Dx()/2 {
		t.Errorf("busy cells cover %dpx of the bar's %dpx travel", covered, bar.Dx())
	}
	if top := a.Cells[0]; top.Box[1] > bar.Max.Y || top.Box[3] < bar.Min.Y {
		t.Errorf("busiest cell is %v, which does not overlap the progress bar at y %d..%d",
			top.Box, bar.Min.Y, bar.Max.Y)
	}

	// The thumbnail flickers in one place for 0.4s, far from the bar.
	thumb := s.Events[2].Region
	found := false
	for _, c := range a.Cells {
		if c.Box[0] < thumb.Max.X && c.Box[2] > thumb.Min.X && c.Box[1] < thumb.Max.Y && c.Box[3] > thumb.Min.Y {
			found = true
			if c.PeakAt < s.Events[2].Start-1 || c.PeakAt > s.Events[2].End+1 {
				t.Errorf("cell %s over the thumbnail peaks at %.2fs, outside the %.2f..%.2f flicker",
					c.Cell, c.PeakAt, s.Events[2].Start, s.Events[2].End)
			}
		}
	}
	if !found {
		t.Errorf("no cell covers the thumbnail at %v, which flickers for 0.4s", thumb)
	}
}

// A cell's activity is a share of that cell, so it cannot exceed the cell.
func TestActivitySharesAreFractions(t *testing.T) {
	for _, s := range []fixture.Scenario{fixture.Reference(), fixture.Player(), fixture.Layout(), fixture.Defect()} {
		for _, c := range activityOf(t, s).Cells {
			if c.Peak > 1 || c.Peak < 0 {
				t.Errorf("%s: cell %s peaked at %.4f of itself", s.Name, c.Cell, c.Peak)
			}
			if c.Share > 1 || c.Share < 0 {
				t.Errorf("%s: cell %s busy for %.2f of the interval", s.Name, c.Cell, c.Share)
			}
			for _, r := range c.Ranges {
				if r[1] < r[0] {
					t.Errorf("%s: cell %s has range %v ending before it starts", s.Name, c.Cell, r)
				}
			}
		}
	}
}

// A cut moves the whole frame, which locates nothing. Reporting it as
// forty-eight equally busy cells buries the reader; it belongs in frame_wide.
func TestActivitySeparatesWholeFrameChangeFromPlaces(t *testing.T) {
	a := activityOf(t, fixture.Reference())
	if len(a.FrameWide) == 0 {
		t.Fatal("the reference scenario cuts between scenes; no frame-wide stretch was reported")
	}
	for _, c := range a.Cells {
		if c.Share > 0.9 {
			t.Errorf("cell %s is busy for %.2f of the recording, which is frame-wide change leaking into the places",
				c.Cell, c.Share)
		}
	}
}

// Cells are off by default: they answer a different question from the
// timeline's, and they are the one output longer than its subject.
func TestActivityIsOptIn(t *testing.T) {
	s := fixture.Player()
	a, err := engine.New(s.Decoder()).Analyse(context.Background(), engine.Defaults("player.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Cells) != 0 || a.Grid != "" {
		t.Errorf("timeline carried %d cells and grid %q without being asked", len(a.Cells), a.Grid)
	}
}
