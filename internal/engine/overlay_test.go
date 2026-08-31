package engine_test

import (
	"context"
	"math"
	"testing"

	"github.com/shhac/agent-motion/internal/engine"
	"github.com/shhac/agent-motion/internal/fixture"
)

func overlayAnalysis(t *testing.T) *engine.Analysis {
	t.Helper()
	s := fixture.Overlay()
	a, err := engine.New(s.Decoder()).Analyse(context.Background(), engine.Defaults("overlay.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// A modal is the case the brightness-map test exists for, and until now no
// fixture drove it end to end — shade_test.go only ever called uniformShade on
// hand-built image pairs, so the pipeline that decides which events are even
// offered to it was untested.
func TestOverlayFixtureRecognisesTheDim(t *testing.T) {
	var found bool
	for _, e := range overlayAnalysis(t).Events {
		if !e.Uniform || e.ShadeScale >= 1 {
			continue
		}
		found = true
		if math.Abs(e.ShadeScale-0.5) > 0.1 {
			t.Errorf("the scrim dims to half; reported scale %.2f", e.ShadeScale)
		}
		if e.RegionArea < 0.9 {
			t.Errorf("a scrim covers the frame; region area %.2f", e.RegionArea)
		}
	}
	if !found {
		t.Error("the page dimming behind a dialog was not identified as an overlay")
	}
}

// Known limitation, pinned rather than asserted.
//
// The same modal closing is reported as the content changing. The transition is
// a dimmed page plus a light dialog returning to a plain page, and a white
// dialog pixel and a dimmed background pixel can land on the same value once
// the dim lifts — so the pair between them has slope zero. At this dialog size
// roughly a third of the sampled pairs are mixed, which is enough to carry the
// median slope onto zero, and the fit is then rejected for being outside the
// scale band.
//
// Measured here: opening uniform=true scale=0.50, closing uniform=false
// scale=0.00. A search that maximises the followed share instead of estimating
// a slope fixes this case, and was tried — it also fitted a real recording's
// undimmed dropdown at scale 0.25 and called it an overlay, so it is not the
// answer yet. Unskip when the estimator is replaced.
func TestOverlayFixtureClosingIsNotYetRecognised(t *testing.T) {
	t.Skip("known limitation: a large light dialog drags the pairwise median on the closing transition")

	var closing bool
	for _, e := range overlayAnalysis(t).Events {
		if e.Start > 5.5 && e.Uniform && e.ShadeScale > 1.5 {
			closing = true
		}
	}
	if !closing {
		t.Error("the modal closing should read as an overlay lifting, not as the content changing")
	}
}
