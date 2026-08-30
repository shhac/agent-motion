package engine

import (
	"context"
	"fmt"
	"image"
	"path/filepath"

	"github.com/shhac/agent-motion/internal/render"
	"github.com/shhac/agent-motion/internal/video"
	output "github.com/shhac/lib-agent-output"
)

// ExtractedFrame is one still written to disk.
type ExtractedFrame struct {
	Time float64 `json:"time_seconds"`
	Path string  `json:"path"`
}

// FrameSet is the result of extracting stills.
type FrameSet struct {
	Input  string           `json:"input"`
	Dir    string           `json:"output_dir"`
	Width  int              `json:"width"`
	Region []int            `json:"region_xyxy,omitempty"`
	Frames []ExtractedFrame `json:"frames"`
	Note   string           `json:"note"`
}

// boxOr falls back to the full frame when no crop was requested.
func boxOr(box image.Rectangle, info video.Info) image.Rectangle {
	if box.Empty() {
		return image.Rect(0, 0, info.Width, info.Height)
	}
	return box
}

// FramesOptions selects which stills to write.
type FramesOptions struct {
	Path   string
	At     []float64
	Dir    string
	Width  int
	Region Region
}

// Frames writes one PNG per requested timestamp. These are real frames, not a
// projection, so they are the right thing to look at once a moment is known.
func (e *Engine) Frames(ctx context.Context, opt FramesOptions) (*FrameSet, error) {
	if len(opt.At) == 0 {
		return nil, output.New("no timestamps requested", output.FixableByAgent).
			WithHint("pass --at 3.4,7.1 or run 'agent-motion timeline' to find moments worth seeing")
	}
	info, err := e.Decoder.Probe(ctx, opt.Path)
	if err != nil {
		return nil, err
	}
	times := sortedUnique(opt.At)
	if err := checkInside(times, info.Duration); err != nil {
		return nil, err
	}
	box := opt.Region.Rect(info)
	set := &FrameSet{
		Input: opt.Path, Dir: opt.Dir, Width: orElse(stillWidth(opt.Width, info.Width, false), info.Width),
		Note: "These are source frames, unmodified apart from scaling.",
	}
	width := stillWidth(opt.Width, boxOr(box, info).Dx(), !box.Empty())
	if !box.Empty() {
		set.Width = orElse(width, box.Dx())
		set.Region = []int{box.Min.X, box.Min.Y, box.Max.X, box.Max.Y}
		set.Note = fmt.Sprintf("These are source frames cropped to %dx%d px at %d,%d and scaled to %d px wide. Apart from the crop and the scale they are unmodified.",
			box.Dx(), box.Dy(), box.Min.X, box.Min.Y, set.Width)
	}
	for _, at := range times {
		raw, err := e.Decoder.Still(ctx, opt.Path, video.Still{At: at, Width: width, Crop: box})
		if err != nil {
			return nil, err
		}
		name := filepath.Join(opt.Dir, fmt.Sprintf("frame-%08.3f.png", at))
		img, err := decodePNG(raw)
		if err != nil {
			return nil, err
		}
		if err := render.Write(name, img); err != nil {
			return nil, err
		}
		set.Frames = append(set.Frames, ExtractedFrame{Time: at, Path: name})
	}
	return set, nil
}
