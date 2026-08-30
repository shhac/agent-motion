package motion

import (
	"image"
	"math"
)

// The bounds separating an overlay from a new picture, calibrated in
// TestUniformShadeSeparatesOverlayFromContent against real recordings: a
// translucent scrim over a news page fits at residual 1.4, while genuine
// content changes measure 20, 33 and 62.
const (
	maxShadeResidual = 8.0
	// A flat starting frame — a blank white page before first paint — can be
	// mapped onto anything, so the fit means nothing. Real content has
	// structure to hold the line down.
	minShadeVariation = 8.0
	// A scrim scales brightness; it does not collapse the picture to a single
	// value or invert it. Outside this band the "fit" is a degenerate one
	// through a nearly uniform frame, which is how a scene cut between two
	// flat-coloured screens sneaks past the residual.
	minShadeScale = 0.25
	maxShadeScale = 3.0
)

// shadeSamples bounds the work: a few thousand points fit a line as well as a
// million and this runs on the largest changes in a recording.
const shadeSamples = 20000

// uniformShade reports whether the after-frame is the before-frame with its
// brightness changed uniformly, rather than a different picture.
//
// This is the difference between a modal backdrop dimming a page and the page
// itself changing, and in the statistics the two are identical: both change
// most of the frame at once and stay changed. An evaluation agent read a
// translucent scrim as a light-to-dark theme flip and only caught it by pulling
// native frames by hand.
//
// An overlay, a dim, and a theme switch all map every pixel through roughly the
// same function, so the after-frame is a straight line in the before-frame. New
// content is not. Fitting that line and measuring what is left over separates
// them for the cost of a few thousand samples.
func uniformShade(before, after image.Image, region image.Rectangle) (residual, scale float64, uniform bool) {
	region = region.Intersect(before.Bounds()).Intersect(after.Bounds())
	if region.Dx() < 8 || region.Dy() < 8 {
		return 0, 0, false
	}
	stride := 1
	if total := region.Dx() * region.Dy(); total > shadeSamples {
		stride = int(math.Sqrt(float64(total) / shadeSamples))
	}

	var n, sumX, sumY, sumXY, sumXX float64
	for y := region.Min.Y; y < region.Max.Y; y += stride {
		for x := region.Min.X; x < region.Max.X; x += stride {
			bx, ay := luma(before, x, y), luma(after, x, y)
			n++
			sumX += bx
			sumY += ay
			sumXY += bx * ay
			sumXX += bx * bx
		}
	}
	denominator := n*sumXX - sumX*sumX
	if n < 64 || denominator == 0 {
		return 0, 0, false
	}
	// The spread of the starting frame. Without it the line is fitted through a
	// flat surface and will match anything.
	spread := math.Sqrt(math.Max(0, sumXX/n-(sumX/n)*(sumX/n)))
	slope := (n*sumXY - sumX*sumY) / denominator
	intercept := (sumY - slope*sumX) / n

	var total float64
	for y := region.Min.Y; y < region.Max.Y; y += stride {
		for x := region.Min.X; x < region.Max.X; x += stride {
			predicted := slope*luma(before, x, y) + intercept
			total += math.Abs(luma(after, x, y) - predicted)
		}
	}
	total /= n
	uniform = total < maxShadeResidual &&
		spread >= minShadeVariation &&
		slope >= minShadeScale && slope <= maxShadeScale
	return round2(total), round2(slope), uniform
}
