package fixture

import "image"

// Overlay is a modal, which is the case the brightness-map test exists for and
// the one hardest to get right: a page dims behind a dialog and then comes
// back. It fades in over ten frames and vanishes in one, because those two
// speeds are classified differently and the tool must say the same thing about
// both — the same modal opening and closing should not read as "the content
// changed" one way and "something translucent laid over it" the other.
func Overlay() Scenario {
	return Scenario{
		Name: "overlay", draw: drawOverlay,
		Width: 640, Height: 360, FPS: 30, Frames: 300,
		Events: []Event{
			{
				Name: "scrim-fade-in", Kind: "overlay", Start: 2.0, End: 2.33,
				Region:      image.Rect(0, 0, 640, 360),
				Description: "The whole page dims to half brightness over ten frames. The content underneath does not change.",
			},
			{
				Name: "dialog", Kind: "overlay", Start: 2.33, End: 6.0,
				Region:      image.Rect(160, 100, 480, 260),
				Description: "A light dialog sits over the dimmed page for about four seconds.",
			},
			{
				Name: "scrim-close", Kind: "overlay", Start: 6.0, End: 6.03,
				Region:      image.Rect(0, 0, 640, 360),
				Description: "The dialog and the dim both vanish in a single frame, restoring the page exactly.",
			},
		},
	}
}

var (
	pageBase = rgb{0xf4, 0xf6, 0xf9}
	pageInk  = rgb{0x22, 0x2a, 0x36}
	pageRule = rgb{0x9a, 0xa6, 0xb8}
	dialogBg = rgb{0xff, 0xff, 0xff}
	dialogIn = rgb{0x1a, 0x20, 0x2a}
)

const (
	overlayOpen  = 60  // frame the fade starts
	overlayFaded = 70  // frame it is fully dim
	overlayClose = 180 // frame everything is restored
)

func drawOverlay(s Scenario, dst []byte, index int) {
	// A page with enough structure that a brightness map has something to hold
	// on to: a flat frame maps onto anything and the fit means nothing.
	fill(dst, s.Width, image.Rect(0, 0, s.Width, s.Height), pageBase)
	for row := 0; row < 9; row++ {
		y := 30 + row*34
		fill(dst, s.Width, image.Rect(40, y, 40+360+row*14, y+12), pageInk)
		fill(dst, s.Width, image.Rect(40, y+18, 600, y+19), pageRule)
	}

	dim := 1.0
	switch {
	case index >= overlayClose:
		dim = 1.0
	case index >= overlayFaded:
		dim = 0.5
	case index >= overlayOpen:
		dim = 1.0 - 0.5*float64(index-overlayOpen)/float64(overlayFaded-overlayOpen)
	}
	if dim < 1.0 {
		scrim(dst, s.Width, s.Height, dim)
	}

	if index >= overlayFaded && index < overlayClose {
		fill(dst, s.Width, image.Rect(160, 100, 480, 260), dialogBg)
		for row := 0; row < 3; row++ {
			y := 130 + row*36
			fill(dst, s.Width, image.Rect(190, y, 450-row*30, y+14), dialogIn)
		}
	}
}

// scrim multiplies every pixel, which is what a translucent black overlay does
// and what the brightness-map fit is looking for.
func scrim(dst []byte, width, height int, factor float64) {
	for i := 0; i < width*height*3; i++ {
		dst[i] = uint8(float64(dst[i]) * factor)
	}
}
