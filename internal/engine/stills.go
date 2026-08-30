package engine

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"slices"
	"strings"

	"github.com/shhac/agent-motion/internal/video"
	output "github.com/shhac/lib-agent-output"
)

func decodePNG(raw []byte) (image.Image, error) {
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, output.Wrap(err, output.FixableByRetry).
			WithHint("the decoder returned bytes that were not a readable PNG")
	}
	return img, nil
}

func checkInside(times []float64, duration float64) error {
	if duration <= 0 {
		return nil
	}
	var bad []string
	for _, t := range times {
		if t < 0 || t >= duration {
			bad = append(bad, fmt.Sprintf("%.2f", t))
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return output.New(fmt.Sprintf("timestamps outside the video: %s", strings.Join(bad, ", ")), output.FixableByAgent).
		WithHint(fmt.Sprintf("the video is %.2fs long; choose timestamps in 0..%.2f", duration, duration))
}

func sortedUnique(in []float64) []float64 {
	out := append([]float64(nil), in...)
	slices.Sort(out)
	return slices.Compact(out)
}

// Region is a crop rectangle in source pixels, optionally padded outwards so a
// thin feature is not flush against the edge of the picture.
type Region struct {
	Box image.Rectangle
	Pad int
}

// Rect returns the padded crop clamped to the frame, or an empty rectangle when
// no region was asked for.
func (r Region) Rect(info video.Info) image.Rectangle {
	if r.Box.Empty() {
		return image.Rectangle{}
	}
	return r.Box.Inset(-r.Pad).Intersect(image.Rect(0, 0, info.Width, info.Height))
}

// stillWidth decides the output width of a still. A full frame is never
// upscaled — a default of 320 should not blow up a small video — but a crop is
// scaled to exactly what was asked for, because magnifying a small region is
// the whole reason to crop one.
func stillWidth(requested, source int, cropped bool) int {
	if requested <= 0 {
		return 0
	}
	if cropped {
		return requested
	}
	if requested >= source {
		return 0
	}
	return requested
}

// orElse returns the resolved width, falling back to the source width when no
// scaling was applied.
func orElse(width, source int) int {
	if width > 0 {
		return width
	}
	return source
}
