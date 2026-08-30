// Package fixture renders a deterministic synthetic scenario whose visual
// events are known exactly. It exists so accumulator and analysis code can be
// tested without a decoder, and so evaluation runs have a ground truth to
// score against.
package fixture

import "image"

// Event is one known visual occurrence in the reference scenario.
type Event struct {
	Name        string          `json:"name"`
	Kind        string          `json:"kind"`
	Start       float64         `json:"start_seconds"`
	End         float64         `json:"end_seconds"`
	Region      image.Rectangle `json:"-"`
	Description string          `json:"description"`
}

// Scenario is a fixed-viewport recording described as pure functions of frame
// index, so any consumer can reproduce it byte for byte.
type Scenario struct {
	Name          string
	Width, Height int
	FPS           float64
	Frames        int
	Events        []Event

	draw func(s Scenario, dst []byte, index int)
}

type rgb struct{ R, G, B uint8 }

var (
	canvas    = rgb{0x1e, 0x24, 0x30}
	header    = rgb{0x2d, 0x36, 0x48}
	sidebar   = rgb{0x26, 0x2e, 0x3d}
	amber     = rgb{0xf2, 0xa8, 0x3b}
	badge     = rgb{0x3d, 0xd6, 0x8c}
	flicker   = rgb{0x4f, 0xd6, 0xe8}
	magenta   = rgb{0xd6, 0x3f, 0xa8}
	altCanvas = rgb{0xe8, 0xe4, 0xda}
	altPanel  = rgb{0xc0, 0xb8, 0xa8}
	white     = rgb{0xff, 0xff, 0xff}
)

// Duration is the scenario length in seconds.
func (s Scenario) Duration() float64 { return float64(s.Frames) / s.FPS }

// Frame writes frame index into dst as rgb24, row major. dst must be
// Width*Height*3 bytes.
func (s Scenario) Frame(dst []byte, index int) { s.draw(s, dst, index) }

func drawReference(s Scenario, dst []byte, index int) {
	t := float64(index) / s.FPS

	if within(t, 21, 21.0+1/s.FPS) {
		fill(dst, s.Width, image.Rect(0, 0, s.Width, s.Height), white)
		return
	}

	if within(t, 15, 18) {
		fill(dst, s.Width, image.Rect(0, 0, s.Width, s.Height), altCanvas)
		fill(dst, s.Width, image.Rect(40, 40, 600, 120), altPanel)
		fill(dst, s.Width, image.Rect(40, 160, 320, 300), altPanel)
		return
	}

	fill(dst, s.Width, image.Rect(0, 0, s.Width, s.Height), canvas)
	fill(dst, s.Width, image.Rect(0, 0, s.Width, 36), header)
	fill(dst, s.Width, image.Rect(0, 36, 120, s.Height), sidebar)

	if within(t, 2, 5) {
		x := 140 + int((t-2)/3*460)
		fill(dst, s.Width, image.Rect(x, 160, x+32, 192), amber)
	}
	if t >= 6.5 {
		fill(dst, s.Width, image.Rect(500, 300, 560, 324), badge)
	}
	if within(t, 9, 12) && (index/3)%2 == 0 {
		fill(dst, s.Width, image.Rect(300, 60, 380, 140), flicker)
	}
	if t >= 23 {
		fill(dst, s.Width, image.Rect(200, 200, 400, 320), mix(canvas, magenta, clamp((t-23)/4)))
	}
}

func within(t, start, end float64) bool { return t >= start && t < end }

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func mix(a, b rgb, f float64) rgb {
	return rgb{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*f),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*f),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*f),
	}
}

// fill paints a rectangle, clipped to the canvas. Clipping here rather than at
// every call site means a scenario can lay content out past the bottom of the
// page — which is what a page pushed down by a layout shift actually does.
func fill(dst []byte, width int, r image.Rectangle, c rgb) {
	height := len(dst) / (width * 3)
	r = r.Intersect(image.Rect(0, 0, width, height))
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := y * width * 3
		for x := r.Min.X; x < r.Max.X; x++ {
			i := row + x*3
			dst[i], dst[i+1], dst[i+2] = c.R, c.G, c.B
		}
	}
}
