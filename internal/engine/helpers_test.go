package engine_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// thumbnailPNG stands in for a decoded still. Its exact content does not
// matter; its size does, because the sheet lays tiles out from it.
func thumbnailPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 160, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 160; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
