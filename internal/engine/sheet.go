package engine

import (
	"context"
	"fmt"
	"image"
	"math"
	"slices"
	"strings"

	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/render"
	"github.com/shhac/agent-motion/internal/video"
)

// SheetTile records what ended up in one cell of a contact sheet.
type SheetTile struct {
	Index int     `json:"index"`
	Time  float64 `json:"time_seconds"`
	Event string  `json:"event,omitempty"`
}

// SheetResult describes a written contact sheet.
type SheetResult struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	// Narrative and Suitability are always present, so a sheet alone still says
	// what kind of recording this is and whether its findings mean anything.
	Narrative   string            `json:"narrative,omitempty"`
	Suitability motion.Assessment `json:"suitability,omitempty"`
	Columns     int               `json:"columns"`
	Thumbnail   int               `json:"thumbnail_width"`
	Region      []int             `json:"region_xyxy,omitempty"`
	Tiles       []SheetTile       `json:"tiles"`
	Chosen      string            `json:"timestamps_chosen_by"`
	HowToRead   string            `json:"how_to_read"`
	// Analysis is included only when the analysis chose the timestamps. With
	// --at the caller already has, or did not want, the full timeline.
	Analysis *Analysis `json:"analysis,omitempty"`
}

// SheetOptions selects the moments and the layout of a contact sheet.
type SheetOptions struct {
	Path    string
	At      []float64
	Count   int
	Columns int
	Width   int
	Output  string
	Region  Region
	// Quick skips the analysis pass. Without it the sheet is analysed even when
	// timestamps are given, so every tile is labelled with the event it lands in.
	Quick   bool
	Analyse AnalyseOptions
}

// Sheet writes one image containing many labelled frames. When no timestamps
// are given it analyses the video first and shows the moments that matter.
func (e *Engine) Sheet(ctx context.Context, opt SheetOptions) (*SheetResult, error) {
	info, err := e.Decoder.Probe(ctx, opt.Path)
	if err != nil {
		return nil, err
	}
	box := opt.Region.Rect(info)
	result := &SheetResult{
		Input: opt.Path, Output: opt.Output,
		Chosen:    "the timestamps you asked for",
		HowToRead: "Frames run left to right, top to bottom. Each caption is the tile number, its timestamp in the source video, and the event it falls inside.",
	}
	if !box.Empty() {
		result.Region = []int{box.Min.X, box.Min.Y, box.Max.X, box.Max.Y}
		result.HowToRead += " Every tile is cropped to the region shown in region_xyxy, so what you see is magnified, not the whole frame."
	}

	times := sortedUnique(opt.At)
	analysis, err := e.sheetAnalysis(ctx, opt)
	if err != nil {
		return nil, err
	}
	if analysis != nil {
		result.Narrative, result.Suitability = analysis.Narrative, analysis.Suitability
		if len(times) == 0 {
			result.Analysis = analysis
			times = chooseTimes(analysis, opt.Count, info.Duration)
			result.Chosen = "the events found by analysing the video, topped up with evenly spaced frames"
		}
	}
	if len(times) == 0 {
		times = evenlySpaced(opt.Count, info.Duration)
		result.Chosen = "evenly spaced across the video"
	}
	if err := checkInside(times, info.Duration); err != nil {
		return nil, err
	}

	tiles, err := e.sheetTiles(ctx, opt, info, box, times, analysis, result)
	if err != nil {
		return nil, err
	}
	if len(tiles) > 0 {
		result.Thumbnail = tiles[0].Image.Bounds().Dx()
	}
	sheet := render.Sheet(tiles, render.SheetOptions{Columns: opt.Columns})
	if err := render.Write(opt.Output, sheet); err != nil {
		return nil, err
	}
	result.Columns = columnsOf(sheet.Bounds().Dx(), tiles)
	return result, nil
}

// sheetAnalysis runs the analysis pass unless the caller opted out. It runs even
// when timestamps were given, because an unlabelled tile makes the reader match
// timestamps to events by hand — and a blank-looking tile is indistinguishable
// from a broken one.
func (e *Engine) sheetAnalysis(ctx context.Context, opt SheetOptions) (*Analysis, error) {
	if opt.Quick {
		return nil, nil
	}
	return e.Analyse(ctx, opt.Analyse)
}

func (e *Engine) sheetTiles(ctx context.Context, opt SheetOptions, info video.Info,
	box image.Rectangle, times []float64, analysis *Analysis, result *SheetResult) ([]render.Tile, error) {

	width := stillWidth(opt.Width, boxOr(box, info).Dx(), !box.Empty())
	tiles := make([]render.Tile, 0, len(times))
	for i, at := range times {
		raw, err := e.Decoder.Still(ctx, opt.Path, video.Still{At: at, Width: width, Crop: box})
		if err != nil {
			return nil, err
		}
		img, err := decodePNG(raw)
		if err != nil {
			return nil, err
		}
		tile := SheetTile{Index: i + 1, Time: at}
		if analysis != nil {
			tile.Event = eventAt(analysis.Events, at)
		}
		result.Tiles = append(result.Tiles, tile)
		tiles = append(tiles, render.Tile{
			Time:  at,
			Label: strings.TrimSpace(fmt.Sprintf("%d  %.2fs  %s", i+1, at, tile.Event)),
			Image: img,
		})
	}
	return tiles, nil
}

// evenlySpaced is the fallback when there is no analysis to choose moments.
func evenlySpaced(count int, duration float64) []float64 {
	if count <= 0 {
		count = 12
	}
	times := make([]float64, 0, count)
	for i := 0; i < count; i++ {
		at := round(duration * (float64(i) + 0.5) / float64(count))
		if duration <= 0 || at < duration {
			times = append(times, at)
		}
	}
	return times
}

func chooseTimes(a *Analysis, count int, duration float64) []float64 {
	if count <= 0 {
		count = 12
	}
	times := append([]float64(nil), a.Inspect...)
	if len(times) > count {
		times = times[:count]
	}
	// Evenly spaced frames fill the gaps so the sheet still describes the whole
	// interval, not only its busiest moments.
	span := a.Params.End - a.Params.Start
	for i := 0; len(times) < count && i < count; i++ {
		at := round(a.Params.Start + span*(float64(i)+0.5)/float64(count))
		if duration > 0 && at >= duration {
			continue
		}
		if !near(times, at, span/float64(count)/2) {
			times = append(times, at)
		}
	}
	return sortedUnique(times)
}

func near(times []float64, at, tolerance float64) bool {
	for _, t := range times {
		if math.Abs(t-at) < tolerance {
			return true
		}
	}
	return false
}

// eventAt names every event covering a timestamp. A moment can be both the end
// of a flicker and the start of the stall that interrupted it, and labelling
// only the first makes the two look like one.
func eventAt(events []motion.Event, at float64) string {
	var kinds []string
	for _, e := range events {
		if at >= e.Start-0.05 && at <= e.End+0.05 && !slices.Contains(kinds, e.Kind) {
			kinds = append(kinds, e.Kind)
		}
	}
	return strings.Join(kinds, "/")
}

func columnsOf(sheetWidth int, tiles []render.Tile) int {
	if len(tiles) == 0 {
		return 0
	}
	cell := tiles[0].Image.Bounds().Dx()
	if cell == 0 {
		return 0
	}
	return max(1, sheetWidth/(cell+6))
}
