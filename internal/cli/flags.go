package cli

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

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
	return engine.AnalyseOptions{
		Path: path, Start: f.start, End: f.end, Threshold: f.threshold,
		Width: f.width, SampleFPS: f.sampleFPS,
		DriftSeconds: f.drift, NoDrift: f.drift == 0,
		MaxEvents: f.maxEvents, Buckets: f.buckets, Native: f.native, Series: f.series,
	}
}

// parseTimes accepts repeated flags and comma-separated lists in either.
func parseTimes(values []string) ([]float64, error) {
	var out []float64
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			t, err := strconv.ParseFloat(part, 64)
			if err != nil {
				return nil, output.New(fmt.Sprintf("could not read %q as a number of seconds", part), output.FixableByAgent).
					WithHint("pass timestamps like --at 3.4,7.1")
			}
			if t < 0 {
				return nil, output.New("timestamps must be zero or greater", output.FixableByAgent)
			}
			out = append(out, t)
		}
	}
	return out, nil
}

// derived builds a sibling path for generated output, e.g. clip.mp4 -> clip.sheet.png.
func derived(input, suffix string) string {
	return strings.TrimSuffix(input, filepath.Ext(input)) + suffix
}
