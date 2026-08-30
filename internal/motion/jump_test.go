package motion_test

import (
	"context"
	"testing"

	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/video"
)

// TestBackwardsJumpInsideMovementIsNamed covers a class of UI fault that is
// otherwise invisible: the movement is real and expected, and the bug is a
// discontinuity in it — a progress bar regressing, a scroll position resetting,
// a carousel snapping back to the start.
func TestBackwardsJumpInsideMovementIsNamed(t *testing.T) {
	const w, h = 320, 180
	// A block slides right, then at frame 60 snaps back to where it was at
	// frame 50 and carries on.
	render := func(dst []byte, index int) {
		for i := range dst {
			dst[i] = 20
		}
		x := 10 + index*3
		if index >= 60 {
			x = 10 + (index-50)*3
		}
		for y := 80; y < 100; y++ {
			for px := x; px < x+16 && px < w; px++ {
				i := (y*w + px) * 3
				dst[i], dst[i+1], dst[i+2] = 240, 200, 60
			}
		}
	}
	dec := &video.Fake{Info: video.Info{Width: w, Height: h, FPS: 30, Duration: 3, NBFrames: 90}, Render: render}
	a := motion.New(w, h, motion.Options{Threshold: 12, Checkpoints: 32, IgnoreAbove: 0.5})
	req := video.Request{Path: "x", Width: w, Height: h, FPS: 30}
	if err := dec.Decode(context.Background(), req, a.Add); err != nil {
		t.Fatal(err)
	}
	timeline := a.Timeline(motion.TimelineOptions{
		FPS: 30, SourceWidth: w, SourceHeight: h, CutFraction: 0.5,
	})

	var jump *motion.Event
	for i := range timeline.Events {
		if timeline.Events[i].JumpPixels > 0 {
			jump = &timeline.Events[i]
		}
	}
	if jump == nil {
		t.Fatalf("the snap back at 2.00s was not reported; got %v", kinds(timeline.Events))
	}
	if jump.Direction != "left to right" {
		t.Errorf("direction = %q, want the overall direction of travel", jump.Direction)
	}
	if jump.JumpSeconds < 1.8 || jump.JumpSeconds > 2.2 {
		t.Errorf("jump reported at %.2fs, want about 2.00s", jump.JumpSeconds)
	}
	if jump.JumpPixels < 40 {
		t.Errorf("jump of %d px is too small for a snap back of about 150 px", jump.JumpPixels)
	}
}

// Smooth movement must not be reported as jumping, or the field means nothing.
func TestSmoothMovementReportsNoJump(t *testing.T) {
	timeline, _ := referenceTimeline(t)
	for _, e := range timeline.Events {
		if e.JumpPixels != 0 {
			t.Errorf("the reference scenario's square moves smoothly, but got %+v", e)
		}
	}
}
