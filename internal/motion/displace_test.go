package motion

import (
	"image"
	"image/color"
	"testing"
)

// page draws a card and a caption at given positions: the two shapes the shift
// detector has to tell apart, one that slides sideways and one that drops.
func page(cardX, captionY int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 640, 360))
	box := func(x0, y0, x1, y1 int, v uint8) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.SetRGBA(x, y, color.RGBA{v, v, v, 0xff})
			}
		}
	}
	box(0, 0, 640, 360, 0x18)
	box(cardX, 120, cardX+122, 160, 0x2c)
	box(60, captionY, 400, captionY+30, 0x8a)
	return img
}

// TestAxisGainSeparatesMovementFromNoise is where minAxisGain comes from. An
// axis that moved gains a lot even for a 2px slide; an axis that did not move
// gains nothing, and gating on the relative improvement there lets codec noise
// invent a phantom sideways shift.
func TestAxisGainSeparatesMovementFromNoise(t *testing.T) {
	before, after := page(200, 240), page(202, 240) // card slid 2px right
	region := image.Rect(200, 120, 324, 160)

	moved := gain(colProfile(before, region), colProfile(after, region))
	still := gain(rowProfile(before, region), rowProfile(after, region))

	if moved <= minAxisGain {
		t.Errorf("a 2px slide gains %.4f, at or below the %.2f threshold — real shifts would be missed",
			moved, minAxisGain)
	}
	if still >= minAxisGain {
		t.Errorf("an axis that did not move gains %.4f, at or above the %.2f threshold — phantom shifts would be invented",
			still, minAxisGain)
	}
	if moved < 2*minAxisGain {
		t.Errorf("only %.4f of margin above the threshold; too tight to survive codec noise", moved)
	}
}

func gain(before, after []float64) float64 {
	zero := profileDistance(before, after, 0)
	best := zero
	limit := min(len(after)/2, 40)
	for d := -limit; d <= limit; d++ {
		if d == 0 {
			continue
		}
		if got := profileDistance(before, after, d); got < best {
			best = got
		}
	}
	return zero - best
}

func TestTranslationMeasuresEachAxisIndependently(t *testing.T) {
	region := image.Rect(40, 100, 440, 300)

	if got := Translation(page(200, 240), page(200, 246), region, 40); got.DY != 6 || got.DX != 0 {
		t.Errorf("a 6px drop measured as %+v, want DY=6 DX=0 — a vertical move must not invent a sideways one", got)
	}
	if got := Translation(page(200, 240), page(206, 240), image.Rect(180, 110, 340, 170), 40); got.DX != 6 || got.DY != 0 {
		t.Errorf("a 6px slide measured as %+v, want DX=6 DY=0", got)
	}
	if got := Translation(page(200, 240), page(200, 240), region, 40); got.Moved() {
		t.Errorf("identical frames reported as moved: %+v", got)
	}
}

// Content that appears must never be reported as content that moved: on a page
// those are a feature and a bug respectively.
func TestAppearingContentIsNotAShift(t *testing.T) {
	before := page(200, 240)
	after := page(200, 240)
	for y := 20; y < 44; y++ {
		for x := 560; x < 620; x++ {
			after.SetRGBA(x, y, color.RGBA{0x3d, 0xd6, 0x8c, 0xff})
		}
	}
	if got := Translation(before, after, image.Rect(540, 10, 640, 60), 20); got.Moved() {
		t.Errorf("a badge appearing was reported as a move of %+v", got)
	}
}

// TestTranslationRefusesWhatItCannotExplain covers the two ways a fit can be
// meaningless, both found on a real browser reflow rather than reasoned about.
//
// A 600x200 image with no dimensions loading late pushed a page down exactly
// 200px. The vertical offset was right, but the same frames also produced a
// confident 426px sideways move that never happened, and first paint — a blank
// page filling with content — was reported as content moving.
func TestTranslationRefusesWhatItCannotExplain(t *testing.T) {
	region := image.Rect(0, 0, 320, 200)

	t.Run("a featureless start cannot be translated", func(t *testing.T) {
		// This is first paint: a blank page has a profile spread of zero, so
		// every offset fits it equally and none of them means anything.
		if got := Translation(flat(255), textured(3), region, 60); got.Moved() {
			t.Errorf("content appearing on a blank page was reported as a move of %+v", got)
		}
	})

	t.Run("an offset that does not explain the change is refused", func(t *testing.T) {
		// New content in part of the region, nothing moved. Some offset will
		// always reduce the difference a little; it must not be believed.
		before := textured(4)
		after := image.NewRGBA(before.Bounds())
		copy(after.Pix, before.Pix)
		for y := 40; y < 120; y++ {
			for x := 30; x < 260; x++ {
				after.SetRGBA(x, y, color.RGBA{0x40, 0x80, 0xc0, 0xff})
			}
		}
		if got := Translation(before, after, region, 60); got.Moved() {
			t.Errorf("content appearing in a region was reported as a move of %+v", got)
		}
	})

	t.Run("a real translation is still measured", func(t *testing.T) {
		before := textured(5)
		after := image.NewRGBA(before.Bounds())
		for y := 0; y < 200; y++ {
			for x := 0; x < 320; x++ {
				src := y - 20
				if src < 0 {
					after.SetRGBA(x, y, color.RGBA{0, 0, 0, 0xff})
					continue
				}
				after.Set(x, y, before.At(x, src))
			}
		}
		got := Translation(before, after, region, 60)
		if got.DY != 20 || got.DX != 0 {
			t.Errorf("a 20px drop measured as %+v, want DY=20 DX=0", got)
		}
	})
}
