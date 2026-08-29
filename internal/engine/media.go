package engine

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/render"
	"github.com/shhac/agent-motion/internal/video"
	output "github.com/shhac/lib-agent-output"
)

// Inspection is the cheap first call: what this file is, without decoding it.
type Inspection struct {
	Input     string     `json:"input"`
	Source    video.Info `json:"source"`
	NextSteps []string   `json:"next_steps"`
}

// Inspect reads container and stream metadata only.
func (e *Engine) Inspect(ctx context.Context, path string) (*Inspection, error) {
	info, err := e.Decoder.Probe(ctx, path)
	if err != nil {
		return nil, err
	}
	return &Inspection{
		Input:  path,
		Source: info,
		NextSteps: []string{
			fmt.Sprintf("agent-motion timeline %s", quote(path)),
			fmt.Sprintf("agent-motion sheet %s", quote(path)),
		},
	}, nil
}

// ExtractedFrame is one still written to disk.
type ExtractedFrame struct {
	Time float64 `json:"time_seconds"`
	Path string  `json:"path"`
}

// FrameSet is the result of extracting stills.
type FrameSet struct {
	Input  string           `json:"input"`
	Dir    string           `json:"output_dir"`
	Width  int              `json:"width"`
	Frames []ExtractedFrame `json:"frames"`
	Note   string           `json:"note"`
}

// FramesOptions selects which stills to write.
type FramesOptions struct {
	Path  string
	At    []float64
	Dir   string
	Width int
}

// Frames writes one PNG per requested timestamp. These are real frames, not a
// projection, so they are the right thing to look at once a moment is known.
func (e *Engine) Frames(ctx context.Context, opt FramesOptions) (*FrameSet, error) {
	if len(opt.At) == 0 {
		return nil, output.New("no timestamps requested", output.FixableByAgent).
			WithHint("pass --at 3.4,7.1 or run 'agent-motion timeline' to find moments worth seeing")
	}
	info, err := e.Decoder.Probe(ctx, opt.Path)
	if err != nil {
		return nil, err
	}
	times := sortedUnique(opt.At)
	if err := checkInside(times, info.Duration); err != nil {
		return nil, err
	}
	set := &FrameSet{
		Input: opt.Path, Dir: opt.Dir, Width: widthOr(opt.Width, info.Width),
		Note: "These are source frames, unmodified apart from scaling.",
	}
	for _, at := range times {
		raw, err := e.Decoder.Still(ctx, opt.Path, at, scaleWidth(opt.Width, info.Width))
		if err != nil {
			return nil, err
		}
		name := filepath.Join(opt.Dir, fmt.Sprintf("frame-%08.3f.png", at))
		img, err := decodePNG(raw)
		if err != nil {
			return nil, err
		}
		if err := render.Write(name, img); err != nil {
			return nil, err
		}
		set.Frames = append(set.Frames, ExtractedFrame{Time: at, Path: name})
	}
	return set, nil
}

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

// ProjectionResult is an analysis plus the activity image it produced.
type ProjectionResult struct {
	*Analysis
	Output    string            `json:"output"`
	Encoding  map[string]string `json:"encoding"`
	Excluded  []float64         `json:"transitions_excluded_from_image,omitempty"`
	HowToRead string            `json:"how_to_read"`
}

// WriteProjection renders the activity image for a completed analysis.
func (e *Engine) WriteProjection(a *Analysis, path string, annotate bool) (*ProjectionResult, error) {
	analyzer := a.Analyzer()
	img := render.Projection(analyzer.Pixels(), render.ProjectionOptions{
		Transitions: analyzer.Accumulated(),
		Annotate:    annotate,
		Caption:     fmt.Sprintf("%s  %.2f-%.2fs", filepath.Base(a.Input), a.Params.Start, a.Params.End),
	})
	if err := render.Write(path, img); err != nil {
		return nil, err
	}
	return &ProjectionResult{
		Analysis: a,
		Output:   path,
		Encoding: map[string]string{
			"red":   "how much that pixel changed in total, scaled against the 99th percentile of this image",
			"green": fmt.Sprintf("when it changed, on average: black is %.2fs and full green is %.2fs", a.Params.Start, a.Params.End),
			"blue":  "how often it changed, raised further when the change kept reversing direction",
			"black": "no change above the threshold at that pixel",
			"alpha": "always opaque, so the black background is real black rather than transparency",
		},
		Excluded: analyzer.Ignored(),
		HowToRead: "Every pixel keeps its source x,y. This is an activity map, not a picture of the video — " +
			"use it to find where and when to look, then read the events and pull real frames.",
	}, nil
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

func decodePNG(raw []byte) (image.Image, error) {
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, output.Wrap(err, output.FixableByRetry).
			WithHint("the decoder returned bytes that were not a readable PNG")
	}
	return img, nil
}

func checkInside(times []float64, duration float64) error {
	if duration <= 0 {
		return nil
	}
	var bad []string
	for _, t := range times {
		if t < 0 || t >= duration {
			bad = append(bad, fmt.Sprintf("%.2f", t))
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return output.New(fmt.Sprintf("timestamps outside the video: %s", strings.Join(bad, ", ")), output.FixableByAgent).
		WithHint(fmt.Sprintf("the video is %.2fs long; choose timestamps in 0..%.2f", duration, duration))
}

func sortedUnique(in []float64) []float64 {
	out := append([]float64(nil), in...)
	sort.Float64s(out)
	kept := out[:0]
	for i, v := range out {
		if i == 0 || v != out[i-1] {
			kept = append(kept, v)
		}
	}
	return kept
}

func scaleWidth(requested, source int) int {
	if requested <= 0 || requested >= source {
		return 0
	}
	return requested
}

func widthOr(requested, source int) int {
	if w := scaleWidth(requested, source); w > 0 {
		return w
	}
	return source
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
