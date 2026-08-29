// Package render turns analysis results into PNG images. It performs no
// decoding and no process execution, so every output is reproducible from
// in-memory inputs.
package render

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	output "github.com/shhac/lib-agent-output"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

var (
	ink      = color.RGBA{0xf2, 0xf2, 0xf2, 0xff}
	panel    = color.RGBA{0x14, 0x16, 0x1a, 0xff}
	hairline = color.RGBA{0x3a, 0x3f, 0x48, 0xff}
)

// Write encodes img to path, creating parent directories and replacing the
// destination only once encoding has fully succeeded.
func Write(path string, img image.Image) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return output.Wrap(err, output.FixableByHuman).
			WithHint("choose an output path in a directory you can write to")
	}
	temp, err := os.CreateTemp(dir, ".agent-motion-*.png")
	if err != nil {
		return output.Wrap(err, output.FixableByHuman).
			WithHint("choose an output path in a directory you can write to")
	}
	name := temp.Name()
	defer func() { _ = os.Remove(name) }()
	if err := png.Encode(temp, img); err != nil {
		_ = temp.Close()
		return output.Wrap(err, output.FixableByRetry)
	}
	if err := temp.Close(); err != nil {
		return output.Wrap(err, output.FixableByRetry)
	}
	if err := os.Rename(name, path); err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}
	return nil
}

// LineHeight is the vertical space one label line occupies.
const LineHeight = 14

// Label draws a single line of text with its baseline at y.
func Label(dst draw.Image, x, y int, text string, c color.Color) {
	d := &font.Drawer{
		Dst: dst, Src: image.NewUniform(c), Face: basicfont.Face7x13,
		Dot: fixed.P(x, y),
	}
	d.DrawString(text)
}

// TextWidth is the pixel width of text in the built-in face.
func TextWidth(text string) int {
	return font.MeasureString(basicfont.Face7x13, text).Round()
}

func fillRect(dst draw.Image, r image.Rectangle, c color.Color) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}
