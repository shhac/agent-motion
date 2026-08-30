// Package video owns the decoder boundary. Everything that runs an external
// process lives here so analysis and rendering can be tested without FFmpeg.
package video

import (
	"context"
	"image"
)

// Info describes a source video as reported by the probe.
type Info struct {
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FPS         float64 `json:"fps"`
	Duration    float64 `json:"duration_seconds"`
	Codec       string  `json:"codec,omitempty"`
	PixelFormat string  `json:"pixel_format,omitempty"`
	BitRate     int64   `json:"bit_rate,omitempty"`
	NBFrames    int     `json:"frames,omitempty"`
	HasAudio    bool    `json:"has_audio"`
	Rotation    int     `json:"rotation,omitempty"`
}

// Request selects an interval and the exact grid the caller wants to analyse.
// Width, Height and FPS are always explicit so decoding is reproducible and a
// consumer never has to guess the shape of the stream it receives.
type Request struct {
	Path          string
	Start, End    float64
	Width, Height int
	FPS           float64
}

// Still selects one frame and how much of it to return. A zero Crop means the
// whole frame; a zero Width means the source width of whatever is returned.
type Still struct {
	At    float64
	Width int
	Crop  image.Rectangle
}

// Frame is one decoded rgb24 image at an absolute source timestamp. Pix is
// reused between callbacks, so a consumer that retains it must copy.
type Frame struct {
	Index int
	Time  float64
	Pix   []byte
}

// Decoder is the seam between the CLI and a locally installed FFmpeg.
type Decoder interface {
	Probe(ctx context.Context, path string) (Info, error)
	Decode(ctx context.Context, req Request, fn func(Frame) error) error
	Still(ctx context.Context, path string, still Still) ([]byte, error)
}
