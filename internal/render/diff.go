package render

import (
	"image"
	"image/color"
	"math"

	"github.com/shhac/agent-motion/internal/motion"
)

// dimmed is how much of the base frame shows through, so what changed reads as
// a highlight over recognisable content rather than as an abstract mask.
const dimmed = 0.30

// Diff draws what changed between two frames: the later frame dimmed, with the
// differing pixels lit in proportion to how much they differ.
//
// Two nearly-identical stills side by side cannot be compared by eye, which is
// exactly the case where the difference matters — a two-pixel shift, a colour a
// shade off. Drawing the difference answers it outright.
func Diff(base image.Image, d motion.Difference) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, d.Width, d.Height))
	origin := base.Bounds().Min
	for y := 0; y < d.Height; y++ {
		for x := 0; x < d.Width; x++ {
			r, g, b, _ := base.At(origin.X+x, origin.Y+y).RGBA()
			out := color.RGBA{
				R: uint8(float64(r>>8) * dimmed),
				G: uint8(float64(g>>8) * dimmed),
				B: uint8(float64(b>>8) * dimmed),
				A: 0xff,
			}
			// Square root so a one-shade difference is still visible next to a
			// region that changed completely.
			if delta := float64(d.Delta[y*d.Width+x]); delta > 0 {
				lit := math.Sqrt(delta / 255)
				out.R = level(math.Min(1, float64(out.R)/255+lit))
				out.G = level(math.Min(1, float64(out.G)/255+lit*0.35))
				out.B = level(math.Min(1, float64(out.B)/255+lit*0.75))
			}
			img.SetRGBA(x, y, out)
		}
	}
	return img
}
