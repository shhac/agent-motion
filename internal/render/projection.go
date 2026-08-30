package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/shhac/agent-motion/internal/motion"
)

// ProjectionOptions controls the activity map image.
type ProjectionOptions struct {
	// Transitions is how many frame-to-frame steps contributed, used to
	// normalise the frequency channel.
	Transitions int
	// Annotate appends a legend band below the frame. The band never shifts
	// the frame's own pixels, so image x,y still maps to source x,y.
	Annotate bool
	// Caption is an extra line drawn in the legend band.
	Caption string
	// Omitted names what this image does not show. It belongs in the picture
	// rather than only in the metadata: a reader who glances at the image and
	// sees nothing after a cut would otherwise conclude nothing happened.
	Omitted string
}

// Projection renders per-pixel activity as one spatially aligned image.
//
// Every inactive pixel is written as opaque black. Leaving it transparent
// would let a viewer's own background show through and invert the reading.
func Projection(s motion.PixelStats, opt ProjectionOptions) *image.RGBA {
	height := s.Height
	if opt.Annotate {
		height += legendHeight
	}
	img := image.NewRGBA(image.Rect(0, 0, s.Width, height))
	fillRect(img, img.Bounds(), color.RGBA{A: 0xff})

	scale := percentile(s.Magnitude, 0.99)
	span := s.End - s.Start
	transitions := math.Max(1, float64(opt.Transitions))

	for p, magnitude := range s.Magnitude {
		if magnitude <= 0 {
			continue
		}
		// Square root keeps low-amplitude but real UI motion visible next to
		// one very bright region.
		red := level(math.Sqrt(math.Min(1, magnitude/scale)))
		green := uint8(0)
		if span > 0 {
			mean := s.WeightedTime[p] / magnitude
			green = level(math.Min(1, math.Max(0, (mean-s.Start)/span)))
		}
		blue := level(math.Sqrt(math.Min(1, float64(s.Changes[p]+s.Reversals[p])/transitions)))
		img.SetRGBA(p%s.Width, p/s.Width, color.RGBA{R: red, G: green, B: blue, A: 0xff})
	}
	if opt.Annotate {
		drawLegend(img, s, opt)
	}
	return img
}

const legendHeight = 74

func drawLegend(img *image.RGBA, s motion.PixelStats, opt ProjectionOptions) {
	top := s.Height
	fillRect(img, image.Rect(0, top, s.Width, top+legendHeight), panel)
	fillRect(img, image.Rect(0, top, s.Width, top+1), hairline)

	// The gradient shows exactly how green maps back to a timestamp.
	barTop, barBottom := top+8, top+18
	barLeft, barRight := 8, s.Width-8
	for x := barLeft; x < barRight; x++ {
		f := float64(x-barLeft) / float64(barRight-barLeft-1)
		for y := barTop; y < barBottom; y++ {
			img.SetRGBA(x, y, color.RGBA{G: level(f), A: 0xff})
		}
	}
	Label(img, barLeft, top+30, fmt.Sprintf("%.2fs", s.Start), ink)
	right := fmt.Sprintf("%.2fs", s.End)
	Label(img, barRight-TextWidth(right), top+30, right, ink)
	Label(img, barLeft, top+44, "green=when  red=how much  blue=how often", ink)
	Label(img, barLeft, top+56, clip(opt.Caption, s.Width-16), ink)
	if opt.Omitted != "" {
		Label(img, barLeft, top+68, clip(opt.Omitted, s.Width-16), warn)
	}
}

// clip shortens text to fit, cutting on runes so a multi-byte character is
// never split, and marking that something was dropped.
func clip(text string, width int) string {
	if TextWidth(text) <= width {
		return text
	}
	runes := []rune(text)
	limit := max(0, width/7-1)
	if limit >= len(runes) {
		return text
	}
	return string(runes[:limit]) + "…"
}

// level converts a 0..1 fraction to an 8-bit channel value.
func level(f float64) uint8 { return uint8(math.Round(255 * math.Min(1, math.Max(0, f)))) }

func percentile(values []float64, q float64) float64 {
	kept := make([]float64, 0, len(values))
	for _, v := range values {
		if v > 0 {
			kept = append(kept, v)
		}
	}
	if len(kept) == 0 {
		return 1
	}
	sort.Float64s(kept)
	i := int(math.Ceil(float64(len(kept))*q)) - 1
	if i < 0 {
		i = 0
	}
	return math.Max(kept[i], 1e-9)
}
