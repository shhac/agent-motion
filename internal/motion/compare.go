package motion

import (
	"image"
	"math"
)

// Difference is the result of comparing two frames directly.
//
// The tool otherwise only ever compares adjacent frames or frames one drift
// window apart. Answering "is this the same as it was earlier" — did the screen
// come back, did the region really revert, is anything at all different between
// these two moments — needs an arbitrary pair, and needs an exact count rather
// than a classification.
type Difference struct {
	Width, Height int
	// Changed is the number of pixels differing by more than the threshold.
	Changed int
	// MaxDelta is the largest single-pixel difference, on the 0..255 scale, so
	// "nothing above the threshold" can be told from "nothing at all".
	MaxDelta float64
	// MeanDelta is averaged over every pixel, not only the changed ones.
	MeanDelta float64
	// Box bounds the changed pixels.
	Box image.Rectangle
	// Delta is the per-pixel difference, for drawing.
	Delta []uint8
}

// Total is the pixel count of the compared area.
func (d Difference) Total() int { return d.Width * d.Height }

// Fraction is the share of the area that changed.
func (d Difference) Fraction() float64 {
	if d.Total() == 0 {
		return 0
	}
	return float64(d.Changed) / float64(d.Total())
}

// Identical reports that not one pixel differs at all.
func (d Difference) Identical() bool { return d.MaxDelta == 0 }

// Compare measures the difference between two images of the same size.
func Compare(a, b image.Image, threshold float64) Difference {
	bounds := a.Bounds().Intersect(b.Bounds())
	w, h := bounds.Dx(), bounds.Dy()
	d := Difference{Width: w, Height: h, Delta: make([]uint8, w*h)}
	if w == 0 || h == 0 {
		return d
	}
	minX, minY, maxX, maxY := w, h, -1, -1
	var total float64

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ar, ag, ab, _ := a.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			br, bg, bb, _ := b.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			delta := (math.Abs(float64(ar>>8)-float64(br>>8)) +
				math.Abs(float64(ag>>8)-float64(bg>>8)) +
				math.Abs(float64(ab>>8)-float64(bb>>8))) / 3
			total += delta
			d.MaxDelta = math.Max(d.MaxDelta, delta)
			d.Delta[y*w+x] = uint8(math.Min(255, delta))
			if delta <= threshold {
				continue
			}
			d.Changed++
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
		}
	}
	d.MeanDelta = total / float64(w*h)
	if maxX >= 0 {
		d.Box = image.Rect(minX, minY, maxX+1, maxY+1)
	}
	return d
}
