package motion

import (
	"fmt"
	"image"
	"math"
)

func (a *Analyzer) scale(opt TimelineOptions) (float64, float64) {
	sx, sy := 1.0, 1.0
	if opt.SourceWidth > 0 && a.width > 0 {
		sx = float64(opt.SourceWidth) / float64(a.width)
	}
	if opt.SourceHeight > 0 && a.height > 0 {
		sy = float64(opt.SourceHeight) / float64(a.height)
	}
	return sx, sy
}

func (a *Analyzer) sourceRect(r image.Rectangle, opt TimelineOptions) [4]int {
	sx, sy := a.scale(opt)
	return [4]int{
		int(math.Floor(float64(r.Min.X) * sx)), int(math.Floor(float64(r.Min.Y) * sy)),
		int(math.Ceil(float64(r.Max.X) * sx)), int(math.Ceil(float64(r.Max.Y) * sy)),
	}
}

func areaFraction(r [4]int, opt TimelineOptions) float64 {
	if opt.SourceWidth <= 0 || opt.SourceHeight <= 0 {
		return 0
	}
	return float64((r[2]-r[0])*(r[3]-r[1])) / float64(opt.SourceWidth*opt.SourceHeight)
}

// positionOf names the third of the frame the region sits in, which is easier
// to act on than four numbers.
func positionOf(r [4]int, opt TimelineOptions) string {
	if opt.SourceWidth <= 0 || opt.SourceHeight <= 0 {
		return "frame"
	}
	w, h := float64(opt.SourceWidth), float64(opt.SourceHeight)
	if float64(r[2]-r[0]) > 0.7*w && float64(r[3]-r[1]) > 0.7*h {
		return "whole frame"
	}
	cx := (float64(r[0]) + float64(r[2])) / 2 / w
	cy := (float64(r[1]) + float64(r[3])) / 2 / h
	return third(cy, "top", "middle", "bottom") + " " + third(cx, "left", "centre", "right")
}

func third(v float64, low, mid, high string) string {
	switch {
	case v < 1.0/3:
		return low
	case v < 2.0/3:
		return mid
	default:
		return high
	}
}

func regionSize(r [4]int) string {
	return fmt.Sprintf("%dx%d px at %d,%d", r[2]-r[0], r[3]-r[1], r[0], r[1])
}

func clock(t float64) string { return fmt.Sprintf("%.2fs", t) }
