package engine

import (
	"fmt"
	"path/filepath"

	"github.com/shhac/agent-motion/internal/render"
)

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
