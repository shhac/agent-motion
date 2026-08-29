package projection

import "testing"

func TestAccumulatorEncodesActivityAndReversal(t *testing.T) {
	a := NewAccumulator(2, 1, 10)
	if err := a.Add([]byte{0, 0, 0, 0, 0, 0}, 0); err != nil {
		t.Fatal(err)
	}
	// First pixel brightens, then darkens: two changes and one reversal.
	if err := a.Add([]byte{100, 100, 100, 0, 0, 0}, 1); err != nil {
		t.Fatal(err)
	}
	if err := a.Add([]byte{0, 0, 0, 0, 0, 0}, 2); err != nil {
		t.Fatal(err)
	}
	img, stats, err := a.Image()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Frames != 3 || stats.Coverage != 0.5 || stats.PeakTime != 1 {
		t.Fatalf("stats = %#v", stats)
	}
	active := img.RGBAAt(0, 0)
	if active.R == 0 || active.B == 0 {
		t.Fatalf("active pixel = %#v, want magnitude and repeated activity", active)
	}
	if inactive := img.RGBAAt(1, 0); inactive.R != 0 || inactive.G != 0 || inactive.B != 0 {
		t.Fatalf("inactive pixel = %#v, want black", inactive)
	}
}

func TestAccumulatorNeedsTwoFrames(t *testing.T) {
	a := NewAccumulator(1, 1, 0)
	if err := a.Add([]byte{0, 0, 0}, 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Image(); err == nil {
		t.Fatal("Image succeeded with a single frame")
	}
}

func TestRatio(t *testing.T) {
	fps, err := ratio("30000/1001")
	if err != nil || fps < 29.96 || fps > 29.98 {
		t.Fatalf("ratio = %v, %v", fps, err)
	}
}
