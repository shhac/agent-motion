package cli

import (
	"github.com/shhac/agent-motion/internal/engine"
	output "github.com/shhac/lib-agent-output"
	"github.com/spf13/cobra"
)

// analyseFlags are shared by every command that decodes an interval.
type analyseFlags struct {
	start, end float64
	threshold  float64
	width      int
	sampleFPS  float64
	drift      float64
	maxEvents  int
	buckets    int
	native     bool
	series     bool
}

func (f *analyseFlags) bind(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.Float64Var(&f.start, "start", 0, "Interval start in seconds")
	fs.Float64Var(&f.end, "end", 0, "Interval end in seconds (default: end of video)")
	fs.Float64Var(&f.threshold, "threshold", engine.DefaultThreshold,
		"Ignore per-pixel changes at or below this 0..255 value; lower to see subtler change")
	fs.IntVar(&f.width, "analysis-width", engine.DefaultAnalysisWidth,
		"Downscale to this width before analysing; smaller is faster, larger sees finer detail")
	fs.Float64Var(&f.sampleFPS, "sample-fps", 0,
		"Analyse this many frames per second (default: every frame)")
	fs.Float64Var(&f.drift, "drift", engine.DefaultDriftSeconds,
		"Also compare each frame with the frame this many seconds earlier, to catch slow change; 0 disables")
	fs.IntVar(&f.maxEvents, "max-events", engine.DefaultMaxEvents, "Cap on reported events")
	fs.IntVar(&f.buckets, "buckets", engine.DefaultBuckets, "Resolution of the activity series; 0 omits it")
	fs.BoolVar(&f.native, "native", false, "Analyse at the source resolution instead of --analysis-width")
	fs.BoolVar(&f.series, "series", false, "Include the numeric activity buckets as well as the sparkline")
}

func (f *analyseFlags) validate() error {
	switch {
	case f.start < 0:
		return output.New("--start must be zero or greater", output.FixableByAgent)
	case f.end != 0 && f.end <= f.start:
		return output.New("--end must be greater than --start", output.FixableByAgent)
	case f.threshold < 0 || f.threshold > 255:
		return output.New("--threshold must be in the range 0..255", output.FixableByAgent)
	case f.width < 0:
		return output.New("--analysis-width must be zero or greater", output.FixableByAgent)
	case f.sampleFPS < 0:
		return output.New("--sample-fps must be zero or greater", output.FixableByAgent)
	case f.drift < 0:
		return output.New("--drift must be zero or greater", output.FixableByAgent)
	}
	return nil
}

func (f *analyseFlags) options(path string) engine.AnalyseOptions {
	// Every flag carries its own default, so these values are always explicit.
	return engine.AnalyseOptions{
		Path: path, Start: f.start, End: f.end, Threshold: f.threshold,
		Width: f.width, SampleFPS: f.sampleFPS, DriftSeconds: f.drift,
		MaxEvents: f.maxEvents, Buckets: f.buckets, Native: f.native, Series: f.series,
	}
}
