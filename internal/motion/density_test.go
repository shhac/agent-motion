package motion

import "testing"

// TestDensitySeparatesChangeFromCodecNoise is where minDensity comes from.
//
// Real recordings are lossily compressed, and a static page re-encoded to mp4
// still has a handful of pixels differing between any two frames — scattered
// across most of the picture. Something that genuinely changed is solid within
// its own bounds. The two are orders of magnitude apart, which is what makes
// the ratio safe to threshold.
func TestDensitySeparatesChangeFromCodecNoise(t *testing.T) {
	cases := []struct {
		name      string
		changed   float64 // share of the frame whose pixels differed
		region    float64 // share of the frame the event covers
		scattered bool
	}{
		// Measured on a static Wikipedia article re-encoded to mp4: 31 pixels
		// of 1,024,000 differing, spread over a 949x557 box.
		{"codec noise on a static page", 31.0 / 1024000, (949.0 * 557) / 1024000, true},
		// Measured on the defect fixture: a 2px card edge sliding, 160 pixels
		// of 230,400 inside a 124x40 box.
		{"a 2px card edge sliding", 160.0 / 230400, (124.0 * 40) / 230400, false},
		{"a badge appearing whole", 1440.0 / 480000, 1440.0 / 480000, false},
	}
	for _, c := range cases {
		e := Event{Kind: KindBlip, PeakChanged: c.changed, RegionArea: c.region}
		if got := scattered(e); got != c.scattered {
			t.Errorf("%s: density %.5f scattered=%v, want %v (threshold %.3f)",
				c.name, c.changed/c.region, got, c.scattered, minDensity)
		}
	}
}

// Only brief events are judged on density. Movement and sustained activity are
// legitimately diffuse — a small object crossing the frame covers a large
// region while changing little of it at any instant.
func TestDiffuseKindsAreNotJudgedOnDensity(t *testing.T) {
	for _, kind := range []string{KindMotion, KindBusy, KindFlicker, KindGradual, KindCut} {
		e := Event{Kind: kind, PeakChanged: 0.00001, RegionArea: 0.5}
		if scattered(e) {
			t.Errorf("%s was rejected as scattered; only step and blip are judged this way", kind)
		}
	}
}

// The slow window holds a full frame per frame of lag, which is by far the
// largest allocation. A full second on a large source would retain hundreds of
// megabytes, so it is bounded — and the caller is told the window it got.
func TestSlowWindowIsBoundedByMemory(t *testing.T) {
	cases := []struct {
		name       string
		want       int
		pixels     int
		expectFull bool
	}{
		{"720p at 30fps", 30, 1280 * 720, true},
		{"1080p at 60fps", 60, 1920 * 1080, false},
		{"4K at 30fps", 30, 3840 * 2160, false},
	}
	for _, c := range cases {
		got := boundedLag(c.want, c.pixels)
		if c.expectFull && got != c.want {
			t.Errorf("%s: window cut to %d frames when %d fits in the budget", c.name, got, c.want)
		}
		if !c.expectFull && got >= c.want {
			t.Errorf("%s: kept the full %d frames, which is %d MB of retained video",
				c.name, got, got*c.pixels*3>>20)
		}
		if got < 2 {
			t.Errorf("%s: window of %d frames is too short to compare anything", c.name, got)
		}
		if bytes := got * c.pixels * 3; bytes > driftBudget {
			t.Errorf("%s: %d MB exceeds the %d MB budget", c.name, bytes>>20, driftBudget>>20)
		}
	}
}
