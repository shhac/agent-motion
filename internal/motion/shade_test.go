package motion

import (
	"image"
	"image/color"
	"math/rand"
	"testing"
)

// textured builds a frame with real structure, which is what a line has to be
// fitted through for the fit to mean anything.
func textured(seed int64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	noise := rand.New(rand.NewSource(seed))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			v := uint8(noise.Intn(200) + 20)
			img.SetRGBA(x, y, color.RGBA{v, v, v, 0xff})
		}
	}
	return img
}

// dimmed returns the same picture with a translucent black scrim over it: every
// pixel through the same brightness map, which is what a modal backdrop does.
func dimmed(src *image.RGBA, factor float64) *image.RGBA {
	out := image.NewRGBA(src.Bounds())
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			r, g, b, _ := src.At(x, y).RGBA()
			out.SetRGBA(x, y, color.RGBA{
				uint8(float64(r>>8) * factor),
				uint8(float64(g>>8) * factor),
				uint8(float64(b>>8) * factor), 0xff})
		}
	}
	return out
}

func flat(v uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			img.SetRGBA(x, y, color.RGBA{v, v, v, 0xff})
		}
	}
	return img
}

// TestUniformShadeSeparatesOverlayFromContent is the calibration. In the
// statistics a modal backdrop and a new screen are identical — both change most
// of the frame at once and stay changed — and an evaluation agent read a
// translucent scrim as a light-to-dark theme flip, catching it only by pulling
// native frames by hand.
func TestUniformShadeSeparatesOverlayFromContent(t *testing.T) {
	region := image.Rect(0, 0, 320, 200)
	page := textured(1)

	t.Run("a scrim over the page is uniform", func(t *testing.T) {
		residual, scale, uniform := uniformShade(page, dimmed(page, 0.5), region)
		if !uniform {
			t.Errorf("a 50%% dim measured residual %.2f scale %.2f and was not called uniform", residual, scale)
		}
		if scale < 0.4 || scale > 0.6 {
			t.Errorf("scale = %.2f, want about 0.5 so the caller can say how far it dimmed", scale)
		}
	})

	t.Run("a different picture is not uniform", func(t *testing.T) {
		if _, _, uniform := uniformShade(page, textured(2), region); uniform {
			t.Error("two unrelated pictures were called a brightness change")
		}
	})

	t.Run("a flat starting frame cannot be fitted", func(t *testing.T) {
		// A blank page before first paint maps onto anything, so the fit is
		// meaningless however small its residual.
		if _, _, uniform := uniformShade(flat(255), page, region); uniform {
			t.Error("a fit through a featureless frame was trusted")
		}
	})

	t.Run("collapsing to one colour is not a brightness change", func(t *testing.T) {
		if _, _, uniform := uniformShade(page, flat(255), region); uniform {
			t.Error("the picture being replaced by solid white was called an overlay")
		}
	})
}
