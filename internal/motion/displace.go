package motion

import (
	"image"
	"math"
)

// Displacement is an axis-aligned translation of content between two frames,
// in analysis pixels.
type Displacement struct {
	DX, DY int
	// Confidence is how much better the translated match is than assuming
	// nothing moved: 0 means no better at all, 1 means a perfect match against
	// a completely different starting point.
	Confidence float64
}

// Moved reports whether anything actually moved.
func (d Displacement) Moved() bool { return d.DX != 0 || d.DY != 0 }

// minShiftConfidence is how much better a translated match must be before the
// change is called a move rather than an appearance. Content that genuinely
// slid produces a near-perfect match at its offset; content that appeared
// produces no improvement at any offset.
const minShiftConfidence = 0.55

// minAxisGain is how much better, in luminance units, the best offset must be
// than not moving at all.
//
// The gate is on the absolute gain rather than on the starting difference,
// because the two are worlds apart. An axis that genuinely moved gains a lot
// even for a tiny shift — a 2px slide of a 122px block gains 0.65 — while an
// axis that did not move starts at almost zero, where the relative improvement
// is meaningless and codec noise alone invents a large phantom offset. Measured
// in TestAxisGainSeparatesMovementFromNoise.
const minAxisGain = 0.25

// A translation leaves almost nothing behind: after[i] is before[i-d], so what
// is left over should be small next to the variation in the signal itself.
// Measured on a real browser reflow — a 600x200 image with no dimensions
// loading late, pushing the page down exactly 200px — the true vertical offset
// leaves a residual of 0.05 of the profile's spread, while a spurious
// horizontal match on the same frames leaves 3.3 of it.
const maxResidualShare = 0.4

// minProfileSpread is how much variation a profile needs before an offset in it
// means anything. A blank page before first paint has a spread of exactly zero
// and can be "translated" onto anything, which is how content appearing came to
// be reported as content moving.
const minProfileSpread = 2.0

// Translation finds how far the content inside a region moved between two
// frames.
//
// A layout shift is the same content in a new place, which makes this a
// registration problem rather than a detection one — and the tool has no other
// way to tell "this moved" from "this appeared", which is the difference
// between a bug and a page working normally.
//
// It correlates one-dimensional brightness profiles, mean luminance per row and
// per column, rather than matching blocks. That costs O(size + search) instead
// of O(size x search), and it fits the domain: a page is rows of text and
// stacked blocks, and the shifts worth finding are axis aligned.
func Translation(before, after image.Image, region image.Rectangle, limit int) Displacement {
	region = region.Intersect(before.Bounds()).Intersect(after.Bounds())
	if region.Dx() < 4 || region.Dy() < 4 || limit < 1 {
		return Displacement{}
	}
	limitY, limitX := min(limit, region.Dy()/2), min(limit, region.Dx()/2)
	dy, confidenceY := bestOffset(
		rowProfile(before, region), rowProfile(after, region), limitY)
	dx, confidenceX := bestOffset(
		colProfile(before, region), colProfile(after, region), limitX)

	// Take the confidence from whichever axis actually moved. A purely vertical
	// shift leaves the column profile unchanged, and scoring that as "no
	// confidence" would reject the very case this exists for.
	confidence := 0.0
	if dy != 0 {
		confidence = math.Max(confidence, confidenceY)
	}
	if dx != 0 {
		confidence = math.Max(confidence, confidenceX)
	}
	if confidence < minShiftConfidence {
		return Displacement{}
	}
	return Displacement{DX: dx, DY: dy, Confidence: round2(confidence)}
}

// bestOffset finds d minimising the difference between after[i] and
// before[i-d], with d positive meaning the content moved forward along the axis.
func bestOffset(before, after []float64, limit int) (int, float64) {
	zero := profileDistance(before, after, 0)
	bestD, best := 0, zero
	for d := -limit; d <= limit; d++ {
		if d == 0 {
			continue
		}
		if distance := profileDistance(before, after, d); distance < best {
			bestD, best = d, distance
		}
	}
	variation := spread(before)
	switch {
	case bestD == 0, zero-best < minAxisGain:
		return 0, 0
	case variation < minProfileSpread:
		return 0, 0 // nothing to register against; any offset would fit
	case best > maxResidualShare*variation:
		return 0, 0 // the offset does not actually explain the change
	}
	return bestD, 1 - best/zero
}

// profileDistance is the mean absolute difference between two profiles at an
// offset. Averaging rather than summing matters: a larger offset overlaps on
// fewer samples, and an unnormalised sum would make every large shift look like
// a better match than a small one.
func profileDistance(before, after []float64, d int) float64 {
	total, count := 0.0, 0
	for i := range after {
		j := i - d
		if j < 0 || j >= len(before) {
			continue
		}
		total += math.Abs(after[i] - before[j])
		count++
	}
	if count < len(after)/2 {
		return math.MaxFloat64 // too little overlap to mean anything
	}
	return total / float64(count)
}

func rowProfile(frame image.Image, r image.Rectangle) []float64 {
	out := make([]float64, r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		total := 0.0
		for x := r.Min.X; x < r.Max.X; x++ {
			total += luma(frame, x, y)
		}
		out[y-r.Min.Y] = total / float64(r.Dx())
	}
	return out
}

func colProfile(frame image.Image, r image.Rectangle) []float64 {
	out := make([]float64, r.Dx())
	for x := r.Min.X; x < r.Max.X; x++ {
		total := 0.0
		for y := r.Min.Y; y < r.Max.Y; y++ {
			total += luma(frame, x, y)
		}
		out[x-r.Min.X] = total / float64(r.Dy())
	}
	return out
}

func luma(frame image.Image, x, y int) float64 {
	r, g, b, _ := frame.At(x, y).RGBA()
	return (float64(r>>8) + float64(g>>8) + float64(b>>8)) / 3
}

// shiftVerifyThreshold is the per-pixel luminance difference that counts as a
// changed pixel when checking a displacement, matching the analysis default.
const shiftVerifyThreshold = 12.0

// shiftVerifyStride subsamples the check. It is a proportion over hundreds of
// thousands of pixels, and every fourth one settles it.
const shiftVerifyStride = 2

// minShiftImprovement is how much of the difference between the two frames
// undoing the displacement has to remove.
const minShiftImprovement = 0.5

// explainsChange asks the frames themselves whether the offset is real.
//
// A one-dimensional profile cannot tell a true match from a periodic one, and a
// page is deeply periodic: regular line spacing, repeated cards, a column of
// identical rows. Measured on a real page that jump-scrolled 659px — further
// than the profile could search, and far enough that too little of the region
// overlapped to register at all — the correlation instead settled confidently
// on 198px, which is the spacing of the repeated block it locked onto.
//
// So the offset is checked against the pixels rather than the summary of them:
// undoing a real displacement makes most of the difference between the two
// frames go away, and undoing a coincidence does not.
func explainsChange(before, after image.Image, region image.Rectangle, d Displacement) bool {
	still, shifted, counted := 0, 0, 0
	for y := region.Min.Y; y < region.Max.Y; y += shiftVerifyStride {
		sy := y - d.DY
		if sy < region.Min.Y || sy >= region.Max.Y {
			continue
		}
		for x := region.Min.X; x < region.Max.X; x += shiftVerifyStride {
			sx := x - d.DX
			if sx < region.Min.X || sx >= region.Max.X {
				continue
			}
			counted++
			here := luma(after, x, y)
			if math.Abs(here-luma(before, x, y)) > shiftVerifyThreshold {
				still++
			}
			if math.Abs(here-luma(before, sx, sy)) > shiftVerifyThreshold {
				shifted++
			}
		}
	}
	if counted == 0 || still == 0 {
		return false
	}
	return float64(shifted) <= (1-minShiftImprovement)*float64(still)
}
