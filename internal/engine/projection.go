package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/render"
)

// ProjectionResult is an analysis plus the activity image it produced.
type ProjectionResult struct {
	*Analysis
	Output    string            `json:"output"`
	Encoding  map[string]string `json:"encoding"`
	Excluded  []float64         `json:"transitions_excluded_from_image,omitempty"`
	Omitted   string            `json:"omitted_from_image,omitempty"`
	HowToRead string            `json:"how_to_read"`
}

// WriteProjection renders the activity image for a completed analysis.
func (e *Engine) WriteProjection(a *Analysis, path string, annotate bool) (*ProjectionResult, error) {
	analyzer := a.Analyzer()
	omitted, short := omissions(a, analyzer.Ignored())
	img := render.Projection(analyzer.Pixels(), render.ProjectionOptions{
		Transitions: analyzer.Accumulated(),
		Annotate:    annotate,
		Caption:     fmt.Sprintf("%s  %.2f-%.2fs", filepath.Base(a.Input), a.Params.Start, a.Params.End),
		Omitted:     short,
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
		Omitted:  omitted,
		HowToRead: "Every pixel keeps its source x,y. This is an activity map, not a picture of the video — " +
			"use it to find where and when to look, then read the events and pull real frames. " +
			"Read omitted_from_image before concluding that nothing happened somewhere.",
	}, nil
}

// omissions names the events this image cannot show, as a sentence for the
// result and a short line for the legend band. It belongs in the picture as
// well as the metadata: a reader who glances at the image and sees nothing
// after a cut would otherwise conclude nothing happened there.
func omissions(a *Analysis, excluded []float64) (full, short string) {
	gradual, stalls := 0, 0
	for _, e := range a.Events {
		switch e.Kind {
		case motion.KindGradual:
			gradual++
		case motion.KindStall:
			stalls++
		}
	}
	var long, brief []string
	if n := len(excluded); n > 0 {
		long = append(long, plural(n, "whole-frame transition")+" excluded so they could not flatten everything else")
		brief = append(brief, fmt.Sprintf("%d cut/flash", n))
	}
	if gradual > 0 {
		long = append(long, plural(gradual, "gradual event")+" that never clears the frame-to-frame threshold")
		brief = append(brief, fmt.Sprintf("%d gradual", gradual))
	}
	if stalls > 0 {
		long = append(long, plural(stalls, "stall")+", which is an absence of change and cannot be drawn")
		brief = append(brief, fmt.Sprintf("%d stall", stalls))
	}
	if len(long) == 0 {
		return "", ""
	}
	return "Not in this image: " + strings.Join(long, "; ") + ". Read the events instead.",
		"not in this image: " + strings.Join(brief, ", ") + " - see events"
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
