package motion_test

import (
	"context"
	"math/rand"
	"strings"
	"testing"

	"github.com/shhac/agent-motion/internal/fixture"
	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/video"
)

func assessScenario(t *testing.T, dec *video.Fake) motion.Timeline {
	t.Helper()
	info := dec.Info
	a := motion.New(160, 90, motion.Options{Threshold: 12, DriftFrames: 15, Checkpoints: 32, IgnoreAbove: 0.5})
	req := video.Request{Path: "x", Width: 160, Height: 90, FPS: info.FPS}
	if err := dec.Decode(context.Background(), req, a.Add); err != nil {
		t.Fatal(err)
	}
	return a.Timeline(motion.TimelineOptions{
		FPS: info.FPS, SourceWidth: info.Width, SourceHeight: info.Height,
		DriftSeconds: 0.5, CutFraction: 0.5,
	})
}

func TestFixedViewportFootageIsCalledSuitable(t *testing.T) {
	timeline := assessScenario(t, fixture.Reference().Decoder())
	if timeline.Fit.Verdict != motion.FitGood {
		t.Errorf("verdict = %q (%+v), want %q for a mostly static screen",
			timeline.Fit.Verdict, timeline.Fit, motion.FitGood)
	}
}

// A recording where the whole picture moves must be called out, because every
// event it produces is a fragment of one moving scene rather than a finding.
func TestFullFrameMotionIsCalledUnsuitable(t *testing.T) {
	const w, h = 320, 180
	noise := rand.New(rand.NewSource(1))
	texture := make([]byte, w*h*3)
	for i := range texture {
		texture[i] = byte(noise.Intn(256))
	}
	// Pan the texture by two pixels a frame, which is what a moving camera does.
	pan := func(dst []byte, index int) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				sx, sy := (x+index*2)%w, (y+index)%h
				s, d := (sy*w+sx)*3, (y*w+x)*3
				copy(dst[d:d+3], texture[s:s+3])
			}
		}
	}
	timeline := assessScenario(t, &video.Fake{
		Info:   video.Info{Width: w, Height: h, FPS: 30, Duration: 4, NBFrames: 120},
		Render: pan,
	})

	if timeline.Fit.Verdict != motion.FitPoor {
		t.Fatalf("verdict = %q (%+v), want %q", timeline.Fit.Verdict, timeline.Fit, motion.FitPoor)
	}
	if timeline.Fit.Advice == "" {
		t.Error("an unsuitable verdict must say what to do instead")
	}
	for _, e := range timeline.Events {
		if e.RegionArea > 0.6 && e.Kind == motion.KindFlicker {
			t.Errorf("whole-frame motion reported as a flicker, which reads as a finding: %+v", e)
		}
		if e.RegionArea > 0.6 && !strings.Contains(e.Summary, "whole-frame motion") &&
			!strings.Contains(e.Summary, "continuous motion") {
			t.Errorf("whole-frame event should say it is not a localised finding: %q", e.Summary)
		}
	}
}
