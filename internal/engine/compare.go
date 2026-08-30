package engine

import (
	"context"
	"fmt"
	"image"

	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/render"
	"github.com/shhac/agent-motion/internal/video"
	output "github.com/shhac/lib-agent-output"
)

// Comparison answers "is this the same as it was", exactly, for two moments.
type Comparison struct {
	Input     string  `json:"input"`
	First     float64 `json:"first_seconds"`
	Second    float64 `json:"second_seconds"`
	Region    [4]int  `json:"region_xyxy,omitempty"`
	Identical bool    `json:"identical"`
	Changed   int     `json:"changed_pixels"`
	Compared  int     `json:"compared_pixels"`
	Fraction  float64 `json:"changed_fraction"`
	MaxDelta  float64 `json:"max_pixel_delta"`
	MeanDelta float64 `json:"mean_pixel_delta"`
	Threshold float64 `json:"threshold"`
	// Differs bounds the changed pixels, in source coordinates.
	Differs   [4]int `json:"differs_within_xyxy,omitempty"`
	Output    string `json:"output,omitempty"`
	Verdict   string `json:"verdict"`
	HowToRead string `json:"how_to_read,omitempty"`
}

// CompareOptions selects the two moments and where to write the difference.
type CompareOptions struct {
	Path      string
	At        []float64
	Threshold float64
	Region    Region
	Output    string
}

// Compare measures how much two moments of a video differ, and optionally draws
// it. Everything else in the tool compares neighbouring frames; this answers a
// question about an arbitrary pair — did the screen come back, did that region
// really revert, is anything at all different between these two times.
func (e *Engine) Compare(ctx context.Context, opt CompareOptions) (*Comparison, error) {
	times := sortedUnique(opt.At)
	if len(times) != 2 {
		return nil, output.New("comparing needs exactly two timestamps", output.FixableByAgent).
			WithHint("pass --at 14.9,18.5")
	}
	info, err := e.Decoder.Probe(ctx, opt.Path)
	if err != nil {
		return nil, err
	}
	if err := checkInside(times, info.Duration); err != nil {
		return nil, err
	}
	box := opt.Region.Rect(info)

	frames := make([]image.Image, 0, 2)
	for _, at := range times {
		raw, err := e.Decoder.Still(ctx, opt.Path, video.Still{At: at, Crop: box})
		if err != nil {
			return nil, err
		}
		img, err := decodePNG(raw)
		if err != nil {
			return nil, err
		}
		frames = append(frames, img)
	}

	d := motion.Compare(frames[0], frames[1], opt.Threshold)
	result := &Comparison{
		Input: opt.Path, First: times[0], Second: times[1],
		Identical: d.Identical(), Changed: d.Changed, Compared: d.Total(),
		Fraction: round4(d.Fraction()), MaxDelta: round(d.MaxDelta),
		MeanDelta: round4(d.MeanDelta), Threshold: opt.Threshold,
	}
	offsetX, offsetY := 0, 0
	if !box.Empty() {
		result.Region = [4]int{box.Min.X, box.Min.Y, box.Max.X, box.Max.Y}
		offsetX, offsetY = box.Min.X, box.Min.Y
	}
	if !d.Box.Empty() {
		result.Differs = [4]int{
			d.Box.Min.X + offsetX, d.Box.Min.Y + offsetY,
			d.Box.Max.X + offsetX, d.Box.Max.Y + offsetY,
		}
	}
	result.Verdict = verdict(d, times, opt.Threshold)

	if opt.Output != "" {
		if err := render.Write(opt.Output, render.Diff(frames[1], d)); err != nil {
			return nil, err
		}
		result.Output = opt.Output
		result.HowToRead = "The image is the later frame dimmed, with everything that differs lit up in proportion to how much. Black means identical."
	}
	return result, nil
}

func verdict(d motion.Difference, times []float64, threshold float64) string {
	switch {
	case d.Identical():
		return fmt.Sprintf("The two frames are pixel-identical at %s and %s. Nothing changed at all between them.",
			clock(times[0]), clock(times[1]))
	case d.Changed == 0:
		return fmt.Sprintf("Nothing differs by more than the threshold of %.0f between %s and %s, but they are not identical: the largest single-pixel difference is %.0f/255. That is what codec noise looks like.",
			threshold, clock(times[0]), clock(times[1]), d.MaxDelta)
	default:
		return fmt.Sprintf("%d of %d pixels (%.2f%%) differ between %s and %s, all within %dx%d px. The largest single-pixel difference is %.0f/255.",
			d.Changed, d.Total(), d.Fraction()*100, clock(times[0]), clock(times[1]),
			d.Box.Dx(), d.Box.Dy(), d.MaxDelta)
	}
}

func clock(t float64) string { return fmt.Sprintf("%.2fs", t) }
