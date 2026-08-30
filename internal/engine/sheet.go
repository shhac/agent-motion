package engine

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/render"
)

// SheetTile records what ended up in one cell of a contact sheet.
type SheetTile struct {
	Index int     `json:"index"`
	Time  float64 `json:"time_seconds"`
	Event string  `json:"event,omitempty"`
}

// SheetResult describes a written contact sheet.
type SheetResult struct {
	Input     string      `json:"input"`
	Output    string      `json:"output"`
	Columns   int         `json:"columns"`
	Thumbnail int         `json:"thumbnail_width"`
	Tiles     []SheetTile `json:"tiles"`
	Chosen    string      `json:"timestamps_chosen_by"`
	HowToRead string      `json:"how_to_read"`
	Analysis  *Analysis   `json:"analysis,omitempty"`
}

// SheetOptions selects the moments and the layout of a contact sheet.
type SheetOptions struct {
	Path    string
	At      []float64
	Count   int
	Columns int
	Width   int
	Output  string
	Analyse AnalyseOptions
}

// Sheet writes one image containing many labelled frames. When no timestamps
// are given it analyses the video first and shows the moments that matter.
func (e *Engine) Sheet(ctx context.Context, opt SheetOptions) (*SheetResult, error) {
	info, err := e.Decoder.Probe(ctx, opt.Path)
	if err != nil {
		return nil, err
	}
	result := &SheetResult{
		Input: opt.Path, Output: opt.Output, Thumbnail: opt.Width,
		Chosen:    "the timestamps you passed with --at",
		HowToRead: "Frames run left to right, top to bottom. The caption under each frame is its timestamp in the source video.",
	}
	times := sortedUnique(opt.At)
	if len(times) == 0 {
		analysis, err := e.Analyse(ctx, opt.Analyse)
		if err != nil {
			return nil, err
		}
		result.Analysis = analysis
		times = chooseTimes(analysis, opt.Count, info.Duration)
		result.Chosen = "the events found by analysing the video, topped up with evenly spaced frames"
	}
	if err := checkInside(times, info.Duration); err != nil {
		return nil, err
	}

	tiles := make([]render.Tile, 0, len(times))
	for i, at := range times {
		raw, err := e.Decoder.Still(ctx, opt.Path, at, scaleWidth(opt.Width, info.Width))
		if err != nil {
			return nil, err
		}
		img, err := decodePNG(raw)
		if err != nil {
			return nil, err
		}
		tile := SheetTile{Index: i + 1, Time: at}
		if result.Analysis != nil {
			tile.Event = eventAt(result.Analysis.Events, at)
		}
		result.Tiles = append(result.Tiles, tile)
		tiles = append(tiles, render.Tile{
			Time:  at,
			Label: strings.TrimSpace(fmt.Sprintf("%d  %.2fs  %s", i+1, at, tile.Event)),
			Image: img,
		})
	}
	sheet := render.Sheet(tiles, render.SheetOptions{Columns: opt.Columns})
	if err := render.Write(opt.Output, sheet); err != nil {
		return nil, err
	}
	result.Columns = columnsOf(sheet.Bounds().Dx(), tiles)
	return result, nil
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

func eventAt(events []motion.Event, at float64) string {
	for _, e := range events {
		if at >= e.Start-0.05 && at <= e.End+0.05 {
			return e.Kind
		}
	}
	return ""
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
