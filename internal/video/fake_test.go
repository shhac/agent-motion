package video_test

import (
	"context"
	"testing"

	"github.com/shhac/agent-motion/internal/fixture"
	"github.com/shhac/agent-motion/internal/video"
)

// The fake is test infrastructure, so its own fidelity is worth pinning: if it
// silently ignored the interval or the rate, every test using it would pass
// against behaviour the real decoder does not have.
func TestFakeHonoursIntervalRateAndScale(t *testing.T) {
	s := fixture.Reference()
	dec := &video.Fake{
		Info:   video.Info{Width: s.Width, Height: s.Height, FPS: s.FPS, Duration: s.Duration(), NBFrames: s.Frames},
		Render: s.Frame,
	}
	var times []float64
	var sizes []int
	err := dec.Decode(context.Background(), video.Request{
		Path: "x", Start: 2, End: 4, Width: 160, Height: 90, FPS: 10,
	}, func(f video.Frame) error {
		times = append(times, f.Time)
		sizes = append(sizes, len(f.Pix))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(times) != 20 {
		t.Fatalf("got %d frames, want 2s at 10 fps", len(times))
	}
	if times[0] != 2 || times[19] < 3.8 || times[19] > 4 {
		t.Errorf("timestamps run %v..%v, want 2..just under 4", times[0], times[19])
	}
	for _, size := range sizes {
		if size != 160*90*3 {
			t.Fatalf("frame size = %d, want the requested 160x90 rgb24", size)
		}
	}
}
