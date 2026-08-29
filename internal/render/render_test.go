package render_test

import (
	"image"
	"testing"

	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/render"
)

func stats() motion.PixelStats {
	s := motion.PixelStats{
		Width: 4, Height: 2,
		Magnitude:    make([]float64, 8),
		WeightedTime: make([]float64, 8),
		Changes:      make([]int32, 8),
		Reversals:    make([]int32, 8),
		Start:        0, End: 10,
	}
	s.Magnitude[0], s.WeightedTime[0], s.Changes[0] = 100, 0, 1    // changed at the start
	s.Magnitude[7], s.WeightedTime[7], s.Changes[7] = 50, 50*10, 4 // changed at the end, often
	return s
}

// TestProjectionIsFullyOpaque is the regression guard for the bug that made
// every inactive pixel transparent, so viewers rendered the map on white.
func TestProjectionIsFullyOpaque(t *testing.T) {
	img := render.Projection(stats(), render.ProjectionOptions{Transitions: 4})
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if _, _, _, alpha := img.At(x, y).RGBA(); alpha != 0xffff {
				t.Fatalf("pixel %d,%d has alpha %d, want opaque", x, y, alpha)
			}
		}
	}
}

func TestProjectionEncodesTimeInGreen(t *testing.T) {
	img := render.Projection(stats(), render.ProjectionOptions{Transitions: 4})
	early := img.RGBAAt(0, 0)
	late := img.RGBAAt(3, 1)
	if early.G >= late.G {
		t.Errorf("early green %d should be darker than late green %d", early.G, late.G)
	}
	if early.R == 0 || late.R == 0 {
		t.Error("both active pixels should carry magnitude in red")
	}
	if inactive := img.RGBAAt(1, 0); inactive.R != 0 || inactive.G != 0 || inactive.B != 0 {
		t.Errorf("inactive pixel = %#v, want black", inactive)
	}
}

func TestProjectionLegendAddsRowsBelowTheFrame(t *testing.T) {
	plain := render.Projection(stats(), render.ProjectionOptions{Transitions: 4})
	annotated := render.Projection(stats(), render.ProjectionOptions{Transitions: 4, Annotate: true, Caption: "clip"})
	if annotated.Bounds().Dx() != plain.Bounds().Dx() {
		t.Error("the legend must not change the width, or x no longer maps to source x")
	}
	if annotated.Bounds().Dy() <= plain.Bounds().Dy() {
		t.Error("the legend should add rows below the frame")
	}
	for y := 0; y < plain.Bounds().Dy(); y++ {
		for x := 0; x < plain.Bounds().Dx(); x++ {
			if annotated.RGBAAt(x, y) != plain.RGBAAt(x, y) {
				t.Fatalf("legend changed frame pixel %d,%d", x, y)
			}
		}
	}
}

func TestSheetLaysTilesOutInReadingOrder(t *testing.T) {
	tiles := make([]render.Tile, 6)
	for i := range tiles {
		tiles[i] = render.Tile{
			Time:  float64(i),
			Label: "t",
			Image: image.NewRGBA(image.Rect(0, 0, 40, 30)),
		}
	}
	sheet := render.Sheet(tiles, render.SheetOptions{Columns: 3, Gap: 2})
	if got, want := sheet.Bounds().Dx(), 3*40+4*2; got != want {
		t.Errorf("width = %d, want %d for 3 columns", got, want)
	}
	if rows := 2; sheet.Bounds().Dy() != rows*(30+18)+(rows+1)*2 {
		t.Errorf("height = %d, want two captioned rows", sheet.Bounds().Dy())
	}
}

func TestSheetHandlesNoTiles(t *testing.T) {
	if render.Sheet(nil, render.SheetOptions{}) == nil {
		t.Error("an empty sheet should still be a valid image")
	}
}
