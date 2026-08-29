package video

import "testing"

func TestRatioReadsFFprobeFrameRates(t *testing.T) {
	cases := map[string]float64{
		"30/1":       30,
		"30000/1001": 29.97,
		"25":         25,
	}
	for input, want := range cases {
		got, err := ratio(input)
		if err != nil {
			t.Errorf("ratio(%q): %v", input, err)
			continue
		}
		if got < want-0.01 || got > want+0.01 {
			t.Errorf("ratio(%q) = %v, want about %v", input, got, want)
		}
	}
	if _, err := ratio("30/0"); err == nil {
		t.Error("a zero denominator should be an error, not an infinite frame rate")
	}
}

func TestFitWidthKeepsAspectAndEvenDimensions(t *testing.T) {
	info := Info{Width: 1920, Height: 1080}
	w, h := FitWidth(info, 321)
	if w != 320 {
		t.Errorf("width = %d, want 320: odd sizes break yuv chroma subsampling", w)
	}
	if h != 180 {
		t.Errorf("height = %d, want 180 to preserve 16:9", h)
	}
	if w, h := FitWidth(info, 0); w != 1920 || h != 1080 {
		t.Errorf("a zero max should keep the native size, got %dx%d", w, h)
	}
	if w, h := FitWidth(info, 4000); w != 1920 || h != 1080 {
		t.Errorf("a max wider than the source should not upscale, got %dx%d", w, h)
	}
}

// TestFiltersPinRateAndSize guards determinism: both the rate and the size are
// forced, so frame index always maps to a known timestamp and a known shape.
func TestFiltersPinRateAndSize(t *testing.T) {
	got := filters(Request{Width: 320, Height: 180, FPS: 25})
	if got != "fps=25,scale=320:180" {
		t.Errorf("filters = %q", got)
	}
}

func TestAvailableReportsMissingExecutablesAsHumanFixable(t *testing.T) {
	err := NewFFmpeg("definitely-not-a-real-ffmpeg", "ffprobe").Available()
	if err == nil {
		t.Fatal("expected an error for a missing executable")
	}
	if got := err.Error(); got == "" {
		t.Error("error should name the missing executable")
	}
}
