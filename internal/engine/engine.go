package engine

import (
	"context"
	"fmt"
	"math"

	"github.com/shhac/agent-motion/internal/motion"
	"github.com/shhac/agent-motion/internal/video"
	output "github.com/shhac/lib-agent-output"
)

// Engine runs commands against one decoder.
type Engine struct {
	Decoder video.Decoder
}

// New returns an Engine backed by dec.
func New(dec video.Decoder) *Engine { return &Engine{Decoder: dec} }

// Defaults used when a caller leaves an option at its zero value.
const (
	DefaultThreshold     = 12.0
	DefaultAnalysisWidth = 320
	DefaultDriftSeconds  = 1.0
	DefaultMaxEvents     = 40
	DefaultBuckets       = 60
	// CutFraction is the share of the frame that must change in one step for
	// the transition to count as a boundary rather than as activity.
	CutFraction = 0.5
)

// Source describes the input file.
type Source struct {
	Path string `json:"path"`
	video.Info
}

// Params records exactly how the analysis was performed, so a result can be
// reproduced and so an agent can tell which knob to turn next.
type Params struct {
	Start          float64 `json:"start_seconds"`
	End            float64 `json:"end_seconds"`
	FramesAnalysed int     `json:"frames_analysed"`
	SampleFPS      float64 `json:"sample_fps"`
	Width          int     `json:"analysis_width"`
	Height         int     `json:"analysis_height"`
	Threshold      float64 `json:"threshold"`
	DriftSeconds   float64 `json:"drift_seconds"`
	NoiseFloor     float64 `json:"noise_floor_fraction"`
}

// AnalyseOptions selects an interval and the sensitivity of the pass.
type AnalyseOptions struct {
	Path         string
	Start, End   float64
	Threshold    float64
	Width        int
	SampleFPS    float64
	DriftSeconds float64
	MaxEvents    int
	Buckets      int
	// Native analyses at the source resolution. It is what the image needs and
	// what a caller wants when a few pixels matter.
	Native bool
	// Series includes the numeric activity buckets alongside the sparkline.
	Series bool
}

// Analysis is the result of one pass: what happened, when, and where.
type Analysis struct {
	Input       string            `json:"input"`
	Source      video.Info        `json:"source"`
	Params      Params            `json:"analysis"`
	Narrative   string            `json:"narrative"`
	Suitability motion.Assessment `json:"suitability"`
	Busiest     float64           `json:"busiest_seconds"`
	Coverage    float64           `json:"motion_coverage"`
	Events      []motion.Event    `json:"events"`
	Omitted     int               `json:"events_omitted,omitempty"`
	Quiet       [][2]float64      `json:"quiet_ranges,omitempty"`
	BucketSec   float64           `json:"bucket_seconds,omitempty"`
	Sparkline   string            `json:"activity_sparkline,omitempty"`
	PeakBucket  float64           `json:"activity_sparkline_full_scale,omitempty"`
	Activity    []float64         `json:"activity_by_bucket,omitempty"`
	Inspect     []float64         `json:"timestamps_worth_inspecting,omitempty"`
	NextSteps   []string          `json:"next_steps,omitempty"`
	Limits      []string          `json:"limits"`

	analyzer *motion.Analyzer
}

// Analyzer exposes the accumulated pass for a caller that also wants an image.
func (a *Analysis) Analyzer() *motion.Analyzer { return a.analyzer }

// Analyse decodes the selected interval once and describes it.
func (e *Engine) Analyse(ctx context.Context, opt AnalyseOptions) (*Analysis, error) {
	info, err := e.Decoder.Probe(ctx, opt.Path)
	if err != nil {
		return nil, err
	}
	opt = opt.withDefaults(info)
	start, end := clampInterval(opt.Start, opt.End, info.Duration)
	if info.Duration > 0 && start >= info.Duration {
		return nil, output.New(fmt.Sprintf("--start %.3fs is at or past the %.3fs end of the video", start, info.Duration), output.FixableByAgent).
			WithHint("choose a start inside the video duration")
	}
	width, height := video.FitWidth(info, opt.Width)
	expected := int(math.Ceil((end - start) * opt.SampleFPS))

	analyzer := motion.New(width, height, motion.Options{
		Threshold:      opt.Threshold,
		DriftFrames:    int(math.Round(opt.DriftSeconds * opt.SampleFPS)),
		Checkpoints:    128,
		ExpectedFrames: expected,
		IgnoreAbove:    CutFraction,
	})
	req := video.Request{Path: opt.Path, Start: start, End: end, Width: width, Height: height, FPS: opt.SampleFPS}
	if err := e.Decoder.Decode(ctx, req, analyzer.Add); err != nil {
		return nil, err
	}
	if analyzer.Frames() < 2 {
		return nil, output.New("the selected interval decoded fewer than two frames", output.FixableByAgent).
			WithHint(fmt.Sprintf("widen the interval; at %.3g fps you need at least %.3fs", opt.SampleFPS, 2/opt.SampleFPS))
	}
	timeline := analyzer.Timeline(motion.TimelineOptions{
		FPS: opt.SampleFPS, SourceWidth: info.Width, SourceHeight: info.Height,
		DriftSeconds: opt.DriftSeconds, CutFraction: CutFraction, MaxEvents: opt.MaxEvents,
	})
	overview := analyzer.Overview(timeline, opt.Buckets)
	spanStart, spanEnd := analyzer.Span()

	return &Analysis{
		Input:  opt.Path,
		Source: info,
		Params: Params{
			Start: round(spanStart), End: round(spanEnd), FramesAnalysed: analyzer.Frames(),
			SampleFPS: opt.SampleFPS, Width: width, Height: height,
			Threshold: opt.Threshold, DriftSeconds: opt.DriftSeconds,
			NoiseFloor: timeline.NoiseFloor,
		},
		Narrative:   overview.Narrative,
		Suitability: timeline.Fit,
		Busiest:     overview.Busiest,
		Coverage:    round4(analyzer.Coverage()),
		Events:      timeline.Events,
		Omitted:     timeline.Truncated,
		Quiet:       overview.Quiet,
		BucketSec:   overview.BucketSeconds,
		Sparkline:   overview.Sparkline,
		PeakBucket:  overview.PeakBucket,
		Activity:    seriesOrNil(overview.Activity, opt.Series),
		Inspect:     overview.Inspect,
		NextSteps:   nextSteps(opt.Path, overview, timeline),
		Limits:      limits(width, info.Width, opt.Threshold, timeline.Fit),
		analyzer:    analyzer,
	}, nil
}

func (o AnalyseOptions) withDefaults(info video.Info) AnalyseOptions {
	if o.Threshold <= 0 {
		o.Threshold = DefaultThreshold
	}
	if o.SampleFPS <= 0 {
		o.SampleFPS = info.FPS
	}
	if o.DriftSeconds < 0 {
		o.DriftSeconds = 0
	} else if o.DriftSeconds == 0 {
		o.DriftSeconds = DefaultDriftSeconds
	}
	if o.MaxEvents <= 0 {
		o.MaxEvents = DefaultMaxEvents
	}
	if o.Buckets <= 0 {
		o.Buckets = DefaultBuckets
	}
	if o.Native {
		o.Width = info.Width
	} else if o.Width <= 0 {
		o.Width = DefaultAnalysisWidth
	}
	return o
}

func clampInterval(start, end, duration float64) (float64, float64) {
	if start < 0 {
		start = 0
	}
	if end <= 0 || (duration > 0 && end > duration) {
		end = duration
	}
	return start, end
}

func round(v float64) float64 { return math.Round(v*100) / 100 }

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

func seriesOrNil(values []float64, include bool) []float64 {
	if include {
		return values
	}
	return nil
}
