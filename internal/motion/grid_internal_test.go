package motion

import (
	"image"
	"testing"
)

// The cell a pixel is counted in and the cell whose area it is counted against
// must be the same cell. They were not: the assignment inverted Bounds with
// x*Cols/Width, which is not its inverse when the frame does not divide
// evenly, so a row of pixels landed in the neighbouring cell and a "fraction"
// of 34/33 came out of it.
func TestGridPartitionAgreesWithBounds(t *testing.T) {
	for _, size := range [][2]int{{320, 200}, {320, 180}, {17, 13}, {97, 61}, {640, 360}, {1280, 799}} {
		w, h := size[0], size[1]
		g := newGrid(8, 6, w, h)

		total := 0
		for _, p := range g.Pixels {
			total += p
		}
		if total != w*h {
			t.Errorf("%dx%d: cells cover %d pixels, want %d", w, h, total, w*h)
		}

		counted := make([]int, len(g.Pixels))
		for y := range h {
			for x := range w {
				c := g.Cell(x, y)
				if c < 0 || c >= len(g.Pixels) {
					t.Fatalf("%dx%d: pixel (%d,%d) maps to cell %d of %d", w, h, x, y, c, len(g.Pixels))
				}
				if !image.Pt(x, y).In(g.Bounds(c)) {
					t.Fatalf("%dx%d: pixel (%d,%d) counted in cell %d, whose bounds are %v",
						w, h, x, y, c, g.Bounds(c))
				}
				counted[c]++
			}
		}
		for c, n := range counted {
			if n != g.Pixels[c] {
				t.Errorf("%dx%d: cell %d counts %d pixels against an area of %d; a fraction of it can exceed 1",
					w, h, c, n, g.Pixels[c])
			}
		}
	}
}
