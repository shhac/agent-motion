package engine

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"sort"
	"strings"

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
	sort.Float64s(out)
	kept := out[:0]
	for i, v := range out {
		if i == 0 || v != out[i-1] {
			kept = append(kept, v)
		}
	}
	return kept
}

func scaleWidth(requested, source int) int {
	if requested <= 0 || requested >= source {
		return 0
	}
	return requested
}

func widthOr(requested, source int) int {
	if w := scaleWidth(requested, source); w > 0 {
		return w
	}
	return source
}
