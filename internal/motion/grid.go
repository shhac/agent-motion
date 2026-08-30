package motion

import "image"

// Cell is one grid cell's contribution to one transition. Counts are pixels;
// the box bounds the changed pixels inside the cell, in analysis coordinates.
type Cell struct {
	// M holds one entry per timescale. An array rather than two named field
	// groups: the fast and slow views are two instances of one thing, and
	// writing them out separately meant every reader re-decided which it wanted.
	M [timescales]measure
}

// measure is one timescale's view of a cell: how many pixels differed and the
// bounds of where.
type measure struct {
	Count                  int32
	MinX, MinY, MaxX, MaxY int16
}

// Box returns the changed area within the cell on one timescale, empty when
// nothing changed on it.
func (c Cell) Box(t timescale) image.Rectangle {
	m := c.M[t]
	if m.Count == 0 {
		return image.Rectangle{}
	}
	return image.Rect(int(m.MinX), int(m.MinY), int(m.MaxX)+1, int(m.MaxY)+1)
}

// Count is the pixel count on one timescale.
func (c Cell) Count(t timescale) int32 { return c.M[t].Count }

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
