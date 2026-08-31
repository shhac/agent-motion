package motion

import (
	"image"
	"math"
	"sort"
)

// The bounds separating an overlay from a new picture, calibrated in
// TestUniformShadeSeparatesOverlayFromContent against real recordings: a
// translucent scrim over a news page fits at residual 1.4, while genuine
// content changes measure 20, 33 and 62.
const (
	// A pixel within this many luminance units of the fitted line counts as
	// following it.
	shadeTolerance = 12.0
	// minShadeFit is the share of the frame that must follow the line.
	minShadeFit = 0.75
	// minShadeStrength is how far the brightness map must be from doing
	// nothing. Trimming outliers lets any two frames sharing a lot of unchanged
	// background fit a line of slope one — which says only that most pixels
	// stayed the same, not that the picture was dimmed. An overlay actually
	// changes the brightness.
	minShadeStrength = 0.12
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
// Something translucent laid over the frame maps every pixel through roughly
// the same function, so the after-frame is a straight line in the before-frame.
// New content is not. Fitting that line and measuring how much of the frame
// follows it separates them for the cost of a few thousand samples.
//
// A theme switch deliberately fails this. Dark to light moves the background up
// and the text down, and no single line fits both directions, so it is reported
// as the content changing — which is what it is.
func uniformShade(before, after image.Image, region image.Rectangle) (fit, scale float64, uniform bool) {
	region = region.Intersect(before.Bounds()).Intersect(after.Bounds())
	if region.Dx() < 8 || region.Dy() < 8 {
		return 0, 0, false
	}
	stride := 1
	if total := region.Dx() * region.Dy(); total > shadeSamples {
		stride = int(math.Sqrt(float64(total) / shadeSamples))
	}

	xs := make([]float64, 0, shadeSamples)
	ys := make([]float64, 0, shadeSamples)
	for y := region.Min.Y; y < region.Max.Y; y += stride {
		for x := region.Min.X; x < region.Max.X; x += stride {
			xs = append(xs, luma(before, x, y))
			ys = append(ys, luma(after, x, y))
		}
	}
	if len(xs) < 64 {
		return 0, 0, false
	}
	if spread(xs) < minShadeVariation {
		// A blank page maps onto anything, so the fit would mean nothing.
		return 0, 0, false
	}

	slope, intercept, ok := robustLine(xs, ys)
	if !ok {
		return 0, 0, false
	}

	fitted := 0
	for i := range xs {
		if math.Abs(ys[i]-(slope*xs[i]+intercept)) <= shadeTolerance {
			fitted++
		}
	}
	share := float64(fitted) / float64(len(xs))
	uniform = share >= minShadeFit &&
		slope >= minShadeScale && slope <= maxShadeScale &&
		math.Abs(slope-1) >= minShadeStrength
	return round2(share), round2(slope), uniform
}

// robustLine fits by taking the median slope over many pairs of points, rather
// than least squares.
//
// A modal is two populations at once: most of the frame dimmed, and a dialog
// that appeared. Least squares has no defence against that — the dialog is a
// tight cluster at one end of the range and drags the line into a compromise
// fitting neither, badly enough to hide the dim completely in the closing
// direction. A median is unmoved by a minority however extreme it is, which is
// exactly the property needed here.
//
// Pairs are taken at fixed strides rather than at random, so the result is
// deterministic for the same input — which the whole tool promises.
func robustLine(xs, ys []float64) (slope, intercept float64, ok bool) {
	slopes := make([]float64, 0, len(xs)*len(pairStrides))
	for _, stride := range pairStrides {
		for i := 0; i+stride < len(xs); i++ {
			dx := xs[i+stride] - xs[i]
			// Nearly equal brightness gives an unstable slope and no
			// information about the map.
			if math.Abs(dx) < minPairSpread {
				continue
			}
			slopes = append(slopes, (ys[i+stride]-ys[i])/dx)
		}
	}
	if len(slopes) < 64 {
		return 0, 0, false
	}
	sort.Float64s(slopes)
	slope = slopes[len(slopes)/2]

	offsets := make([]float64, len(xs))
	for i := range xs {
		offsets[i] = ys[i] - slope*xs[i]
	}
	sort.Float64s(offsets)
	return slope, offsets[len(offsets)/2], true
}

// pairStrides spread the sampled pairs across the frame rather than clustering
// them in one region.
var pairStrides = []int{1, 7, 53, 211}

// minPairSpread is the brightness difference a pair needs before its slope
// means anything.
const minPairSpread = 12.0

// spread is the standard deviation of a sample.
func spread(values []float64) float64 {
	var sum, sumSq float64
	for _, v := range values {
		sum += v
		sumSq += v * v
	}
	n := float64(len(values))
	return math.Sqrt(math.Max(0, sumSq/n-(sum/n)*(sum/n)))
}
