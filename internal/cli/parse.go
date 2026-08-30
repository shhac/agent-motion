package cli

// Parsing for the arguments shared across commands. Kept apart from flag
// binding so the rules for reading a timestamp, a region or a window are in
// one place and testable on their own.

import (
	"fmt"
	"image"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shhac/agent-motion/internal/engine"
	output "github.com/shhac/lib-agent-output"
)

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

// parseRegion reads an x0,y0,x1,y1 rectangle, the same four numbers an event
// reports in region_xyxy, so a region can be pasted straight from a result.
func parseRegion(value string, pad int) (engine.Region, error) {
	if strings.TrimSpace(value) == "" {
		return engine.Region{Pad: pad}, nil
	}
	parts := strings.Split(value, ",")
	if len(parts) != 4 {
		return engine.Region{}, output.New("--region needs four numbers: x0,y0,x1,y1", output.FixableByAgent).
			WithHint("paste an event's region_xyxy, e.g. --region 500,300,560,324")
	}
	var n [4]int
	for i, part := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return engine.Region{}, output.New(fmt.Sprintf("could not read %q in --region as a whole number", part), output.FixableByAgent)
		}
		n[i] = v
	}
	if n[2] <= n[0] || n[3] <= n[1] {
		return engine.Region{}, output.New("--region needs x1 greater than x0 and y1 greater than y0", output.FixableByAgent)
	}
	return engine.Region{Box: image.Rect(n[0], n[1], n[2], n[3]), Pad: pad}, nil
}

// parseWindow reads a start:end window, which is how an event's own span is
// pasted straight back in. Sampling a window evenly is otherwise a matter of
// working out a step size by hand for every event.
func parseWindow(value string) (start, end float64, ok bool, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, 0, false, nil
	}
	first, second, found := strings.Cut(value, ":")
	if !found {
		first, second, found = strings.Cut(value, "-")
	}
	if !found {
		return 0, 0, false, output.New("--during needs a window like 13.07:13.40", output.FixableByAgent)
	}
	times, err := parseTimes([]string{first, second})
	if err != nil {
		return 0, 0, false, err
	}
	if len(times) != 2 || times[1] <= times[0] {
		return 0, 0, false, output.New("--during needs an end greater than its start", output.FixableByAgent).
			WithHint("paste an event's start_seconds and end_seconds, e.g. --during 13.07:13.40")
	}
	return times[0], times[1], true, nil
}

// spread returns count timestamps evenly covering a window, ends included.
func spread(start, end float64, count int) []float64 {
	if count < 2 {
		return []float64{start}
	}
	times := make([]float64, count)
	for i := range times {
		times[i] = math.Round((start+(end-start)*float64(i)/float64(count-1))*100) / 100
	}
	return times
}

// derived builds a sibling path for generated output, e.g. clip.mp4 -> clip.sheet.png.
func derived(input, suffix string) string {
	return strings.TrimSuffix(input, filepath.Ext(input)) + suffix
}

// timestamps resolves --at and --during into the moments to sample. --during
// exists because working out a step size by hand for every event was the main
// source of manual iteration for agents using the tool.
func timestamps(at []string, during string, count int) ([]float64, error) {
	start, end, ok, err := parseWindow(during)
	if err != nil {
		return nil, err
	}
	explicit, err := parseTimes(at)
	if err != nil {
		return nil, err
	}
	if !ok {
		return explicit, nil
	}
	if len(explicit) > 0 {
		return nil, output.New("pass either --at or --during, not both", output.FixableByAgent)
	}
	return spread(start, end, count), nil
}
