package motion

import "image"

// Cell is one grid cell's contribution to one transition. Counts are pixels;
// the box bounds the changed pixels inside the cell, in analysis coordinates.
type Cell struct {
	Changed                    int32
	Drift                      int32
	MinX, MinY, MaxX, MaxY     int16
	DMinX, DMinY, DMaxX, DMaxY int16
}

// Box returns the changed area within the cell, empty when nothing changed.
func (c Cell) Box() image.Rectangle {
	if c.Changed == 0 {
		return image.Rectangle{}
	}
	return image.Rect(int(c.MinX), int(c.MinY), int(c.MaxX)+1, int(c.MaxY)+1)
}

// DriftBox is the same for the slow timescale, which is the only bound a
// gradual change has: it never registers as a fast change at all.
func (c Cell) DriftBox() image.Rectangle {
	if c.Drift == 0 {
		return image.Rectangle{}
	}
	return image.Rect(int(c.DMinX), int(c.DMinY), int(c.DMaxX)+1, int(c.DMaxY)+1)
}

// Grid describes the spatial decomposition used for segmentation.
type Grid struct {
	Cols, Rows    int
	Width, Height int
	// Pixels is the pixel count of each cell, which differs at the right and
	// bottom edges when the frame does not divide evenly.
	Pixels []int
}

// Bounds returns the analysis-coordinate rectangle of cell index i.
func (g Grid) Bounds(i int) image.Rectangle {
	col, row := i%g.Cols, i/g.Cols
	return image.Rect(
		col*g.Width/g.Cols, row*g.Height/g.Rows,
		(col+1)*g.Width/g.Cols, (row+1)*g.Height/g.Rows,
	)
}

// Adjacent reports whether two cells touch, including diagonally.
func (g Grid) Adjacent(a, b int) bool {
	dc := abs(a%g.Cols - b%g.Cols)
	dr := abs(a/g.Cols - b/g.Cols)
	return dc <= 1 && dr <= 1
}

func newGrid(cols, rows, width, height int) Grid {
	g := Grid{Cols: cols, Rows: rows, Width: width, Height: height, Pixels: make([]int, cols*rows)}
	for i := range g.Pixels {
		b := g.Bounds(i)
		g.Pixels[i] = max(1, b.Dx()*b.Dy())
	}
	return g
}

// Grid returns the spatial decomposition in use.
func (a *Analyzer) Grid() Grid { return a.grid }
