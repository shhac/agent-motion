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
