package motion

import (
	"image"
	"math"

	"github.com/shhac/agent-motion/internal/video"
)

type checkpoint struct {
	time float64
	pix  []byte
}

func checkpointSize(width, height int) (int, int) {
	const target = 96
	if width <= target {
		return width, height
	}
	h := int(math.Round(float64(height) * target / float64(width)))
	return target, max(1, h)
}

func (a *Analyzer) recordCheckpoint(f video.Frame) {
	if a.opt.Checkpoints <= 0 {
		return
	}
	if a.cpCounter%a.cpStride != 0 {
		a.cpCounter++
		return
	}
	a.cpCounter++
	if len(a.checkpoints) >= a.opt.Checkpoints {
		a.thinCheckpoints()
	}
	pix := make([]byte, a.cpWidth*a.cpHeight*3)
	downsample(f.Pix, a.width, a.height, pix, a.cpWidth, a.cpHeight)
	a.checkpoints = append(a.checkpoints, checkpoint{time: f.Time, pix: pix})
}

// thinCheckpoints halves the retained set and doubles the stride, keeping a
// bounded, evenly spaced history for a stream of unknown length.
func (a *Analyzer) thinCheckpoints() {
	kept := a.checkpoints[:0]
	for i, c := range a.checkpoints {
		if i%2 == 0 {
			kept = append(kept, c)
		}
	}
	a.checkpoints = kept
	a.cpStride *= 2
}

// persists compares the nearest retained frames either side of a stretch to say
// whether the region ended up looking different.
func (a *Analyzer) persists(region image.Rectangle, start, end float64) *bool {
	before, after := a.checkpointBefore(start), a.checkpointAfter(end)
	if before == nil || after == nil || region.Empty() {
		return nil
	}
	r := a.checkpointRect(region)
	if r.Empty() {
		return nil
	}
	var total float64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := (y*a.cpWidth + x) * 3
			total += pixelDelta(before.pix, after.pix, i)
		}
	}
	changed := total/float64(r.Dx()*r.Dy()) > math.Max(4, a.opt.Threshold/2)
	return &changed
}

func (a *Analyzer) checkpointBefore(t float64) *checkpoint {
	for i := len(a.checkpoints) - 1; i >= 0; i-- {
		if a.checkpoints[i].time < t {
			return &a.checkpoints[i]
		}
	}
	return nil
}

func (a *Analyzer) checkpointAfter(t float64) *checkpoint {
	for i := range a.checkpoints {
		if a.checkpoints[i].time > t {
			return &a.checkpoints[i]
		}
	}
	return nil
}

func (a *Analyzer) checkpointRect(r image.Rectangle) image.Rectangle {
	if a.cpWidth == 0 || a.cpHeight == 0 {
		return image.Rectangle{}
	}
	sx := float64(a.cpWidth) / float64(a.width)
	sy := float64(a.cpHeight) / float64(a.height)
	out := image.Rect(
		int(math.Floor(float64(r.Min.X)*sx)), int(math.Floor(float64(r.Min.Y)*sy)),
		int(math.Ceil(float64(r.Max.X)*sx)), int(math.Ceil(float64(r.Max.Y)*sy)),
	).Intersect(image.Rect(0, 0, a.cpWidth, a.cpHeight))
	if out.Dx() == 0 || out.Dy() == 0 {
		return image.Rectangle{}
	}
	return out
}
