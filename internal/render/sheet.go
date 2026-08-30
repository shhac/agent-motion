package render

import (
	"image"
	"image/draw"
)

// Tile is one labelled frame in a contact sheet.
type Tile struct {
	Time  float64
	Label string
	Image image.Image
}

// SheetOptions controls contact-sheet layout.
type SheetOptions struct {
	Columns int
	Gap     int
}

const (
	captionBand = 18
	defaultGap  = 6
)

// Sheet tiles labelled frames into one image in reading order, so an agent can
// see many moments for the cost of a single picture.
func Sheet(tiles []Tile, opt SheetOptions) *image.RGBA {
	if len(tiles) == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	gap := opt.Gap
	if gap <= 0 {
		gap = defaultGap
	}
	columns := opt.Columns
	if columns <= 0 {
		columns = autoColumns(len(tiles))
	}
	cellW, cellH := tiles[0].Image.Bounds().Dx(), tiles[0].Image.Bounds().Dy()
	for _, t := range tiles {
		cellW = max(cellW, t.Image.Bounds().Dx())
		cellH = max(cellH, t.Image.Bounds().Dy())
	}
	rows := (len(tiles) + columns - 1) / columns
	width := columns*cellW + (columns+1)*gap
	height := rows*(cellH+captionBand) + (rows+1)*gap

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	fillRect(img, img.Bounds(), panel)

	for i, t := range tiles {
		col, row := i%columns, i/columns
		x := gap + col*(cellW+gap)
		y := gap + row*(cellH+captionBand+gap)
		draw.Draw(img, image.Rect(x, y, x+cellW, y+cellH), t.Image, t.Image.Bounds().Min, draw.Src)
		// A hairline marks where the frame ends. Without it an all-white frame
		// — a real flash, say — is indistinguishable from a blank tile.
		outline(img, image.Rect(x, y, x+cellW, y+cellH))
		// The caption sits below the frame rather than on it: text drawn over a
		// light frame is unreadable, and an overlay hides source pixels.
		fillRect(img, image.Rect(x, y+cellH, x+cellW, y+cellH+captionBand), panel)
		Label(img, x+2, y+cellH+13, fit(t.Label, cellW-4), ink)
	}
	return img
}

// autoColumns keeps sheets close to square, which reads better than a long strip.
func autoColumns(n int) int {
	switch {
	case n <= 3:
		return n
	case n <= 8:
		return 4
	case n <= 18:
		return 5
	default:
		return 6
	}
}

// outline draws a one-pixel border just inside a rectangle.
func outline(dst *image.RGBA, r image.Rectangle) {
	fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), hairline)
	fillRect(dst, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), hairline)
	fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), hairline)
	fillRect(dst, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), hairline)
}

func fit(text string, width int) string {
	if width <= 0 {
		return ""
	}
	limit := width / 7
	if len(text) <= limit {
		return text
	}
	if limit <= 1 {
		return ""
	}
	return text[:limit-1] + "…"
}
