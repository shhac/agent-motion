// Package projection owns the deterministic v1 transform and the narrow
// FFmpeg process boundary. The accumulator is intentionally decoder-free so
// its behavior can be tested from synthetic frames.
package projection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	output "github.com/shhac/lib-agent-output"
)

// Config describes one selected temporal interval. End zero means the end of
// the source video. Threshold compares the mean absolute RGB delta per pixel.
type Config struct {
	Input, Output   string
	Start, End      float64
	Threshold       uint8
	FFmpeg, FFprobe string
}

// Result is written to stdout as the tool-callable explanation of the PNG.
// Consumers must consult Encoding instead of assuming a conventional RGB image.
type Result struct {
	Input            string            `json:"input"`
	Output           string            `json:"output"`
	Start            float64           `json:"start_seconds"`
	End              float64           `json:"end_seconds"`
	Frames           int               `json:"frames"`
	FPS              float64           `json:"fps"`
	Width            int               `json:"width"`
	Height           int               `json:"height"`
	MotionCoverage   float64           `json:"motion_coverage"`
	PeakActivityTime float64           `json:"peak_activity_time_seconds"`
	Threshold        uint8             `json:"threshold"`
	Encoding         map[string]string `json:"encoding"`
	Decoder          map[string]string `json:"decoder"`
}

type videoInfo struct {
	Width, Height int
	FPS, Duration float64
}

type probeResponse struct {
	Streams []struct {
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// Project decodes RGB frames from FFmpeg, transforms them, atomically writes
// the PNG, then returns metadata. It never contacts a network service.
func Project(ctx context.Context, cfg Config) (Result, error) {
	if err := executable(cfg.FFmpeg, "ffmpeg"); err != nil {
		return Result{}, err
	}
	if err := executable(cfg.FFprobe, "ffprobe"); err != nil {
		return Result{}, err
	}
	info, err := probe(ctx, cfg.FFprobe, cfg.Input)
	if err != nil {
		return Result{}, err
	}
	if info.Width <= 0 || info.Height <= 0 || info.FPS <= 0 {
		return Result{}, output.New("source has no decodable video stream", output.FixableByAgent).
			WithHint("provide a video with a readable video stream")
	}
	end := cfg.End
	if end == 0 && info.Duration > 0 {
		end = info.Duration
	}
	acc := NewAccumulator(info.Width, info.Height, cfg.Threshold)
	if err := decode(ctx, cfg, info, func(frame []byte, index int) error {
		return acc.Add(frame, float64(index)/info.FPS)
	}); err != nil {
		return Result{}, err
	}
	img, stats, err := acc.Image()
	if err != nil {
		return Result{}, err
	}
	if err := writePNG(cfg.Output, img); err != nil {
		return Result{}, err
	}
	if end == 0 { // Unknown source duration: report the interval actually decoded.
		end = cfg.Start + math.Max(0, float64(stats.Frames-1)/info.FPS)
	}
	return Result{
		Input: cfg.Input, Output: cfg.Output, Start: cfg.Start, End: end,
		Frames: stats.Frames, FPS: info.FPS, Width: info.Width, Height: info.Height,
		MotionCoverage: stats.Coverage, PeakActivityTime: cfg.Start + stats.PeakTime,
		Threshold: cfg.Threshold,
		Encoding: map[string]string{
			"red":   "accumulated mean absolute RGB change, normalized by this image's p99 activity",
			"green": "mean normalized time of above-threshold change (dark early, bright late)",
			"blue":  "above-threshold change frequency, increased by luminance sign reversals",
		},
		Decoder: map[string]string{"ffmpeg": cfg.FFmpeg, "ffprobe": cfg.FFprobe, "pixel_format": "rgb24"},
	}, nil
}

func executable(name, kind string) error {
	if _, err := exec.LookPath(name); err != nil {
		return output.New(fmt.Sprintf("%s executable %q was not found", kind, name), output.FixableByHuman).
			WithHint("install FFmpeg or pass --" + kind + " /path/to/" + kind)
	}
	return nil
}

func probe(ctx context.Context, ffprobe, input string) (videoInfo, error) {
	cmd := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height,avg_frame_rate:format=duration", "-of", "json", input)
	raw, err := cmd.Output()
	if err != nil {
		return videoInfo{}, output.Wrap(fmt.Errorf("ffprobe %q: %w", input, err), output.FixableByAgent).
			WithHint("check the input path and that FFprobe can read the video")
	}
	var response probeResponse
	if err := json.Unmarshal(raw, &response); err != nil || len(response.Streams) == 0 {
		return videoInfo{}, output.New("FFprobe returned no readable video stream", output.FixableByAgent).
			WithHint("check the input is a supported video file")
	}
	fps, err := ratio(response.Streams[0].AvgFrameRate)
	if err != nil || fps <= 0 {
		return videoInfo{}, output.New("could not determine a positive source frame rate", output.FixableByAgent)
	}
	duration, _ := strconv.ParseFloat(response.Format.Duration, 64)
	return videoInfo{Width: response.Streams[0].Width, Height: response.Streams[0].Height, FPS: fps, Duration: duration}, nil
}

func ratio(s string) (float64, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("bad ratio %q", s)
	}
	n, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, err
	}
	d, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || d == 0 {
		return 0, fmt.Errorf("bad denominator in %q", s)
	}
	return n / d, nil
}

func decode(ctx context.Context, cfg Config, info videoInfo, receive func([]byte, int) error) error {
	args := []string{"-v", "error", "-ss", strconv.FormatFloat(cfg.Start, 'f', -1, 64), "-i", cfg.Input}
	if cfg.End > 0 {
		args = append(args, "-t", strconv.FormatFloat(cfg.End-cfg.Start, 'f', -1, 64))
	}
	args = append(args, "-an", "-sn", "-dn", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
	cmd := exec.CommandContext(ctx, cfg.FFmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return output.Wrap(err, output.FixableByRetry)
	}
	if err := cmd.Start(); err != nil {
		return output.Wrap(err, output.FixableByRetry)
	}
	frameSize := info.Width * info.Height * 3
	frame := make([]byte, frameSize)
	index := 0
	for {
		_, err := io.ReadFull(stdout, frame)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			_ = cmd.Wait()
			return output.Wrap(err, output.FixableByRetry)
		}
		if err := receive(frame, index); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}
		index++
	}
	if err := cmd.Wait(); err != nil {
		hint := strings.TrimSpace(stderr.String())
		return output.Wrap(fmt.Errorf("ffmpeg decode: %w", err), output.FixableByAgent).WithHint(hint)
	}
	return nil
}

func writePNG(path string, img image.Image) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}
	temp, err := os.CreateTemp(dir, ".agent-motion-*.png")
	if err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()
	if err := png.Encode(temp, img); err != nil {
		_ = temp.Close()
		return output.Wrap(err, output.FixableByRetry)
	}
	if err := temp.Close(); err != nil {
		return output.Wrap(err, output.FixableByRetry)
	}
	if err := os.Rename(tempName, path); err != nil {
		return output.Wrap(err, output.FixableByHuman)
	}
	return nil
}

// Accumulator holds the v1 per-pixel statistics. Frame values are RGB24 in
// row-major order, matching FFmpeg's rawvideo output.
type Accumulator struct {
	width, height int
	threshold     float64
	previous      []byte
	magnitude     []float64
	weightedTime  []float64
	changes       []int
	reversals     []int
	lastSign      []int8
	frames        int
	lastTime      float64
	peakActivity  int
	peakTime      float64
}

type Stats struct {
	Frames   int
	Coverage float64
	PeakTime float64
}

func NewAccumulator(width, height int, threshold uint8) *Accumulator {
	pixels := width * height
	return &Accumulator{width: width, height: height, threshold: float64(threshold), magnitude: make([]float64, pixels), weightedTime: make([]float64, pixels), changes: make([]int, pixels), reversals: make([]int, pixels), lastSign: make([]int8, pixels)}
}

func (a *Accumulator) Add(frame []byte, time float64) error {
	if len(frame) != a.width*a.height*3 {
		return output.New("decoded frame has an unexpected pixel count", output.FixableByRetry)
	}
	if a.frames == 0 {
		a.previous = append([]byte(nil), frame...)
		a.frames++
		return nil
	}
	activity := 0
	for p := range a.magnitude {
		i := p * 3
		dr := math.Abs(float64(frame[i]) - float64(a.previous[i]))
		dg := math.Abs(float64(frame[i+1]) - float64(a.previous[i+1]))
		db := math.Abs(float64(frame[i+2]) - float64(a.previous[i+2]))
		delta := (dr + dg + db) / 3
		if delta > a.threshold {
			a.magnitude[p] += delta
			a.weightedTime[p] += delta * time
			a.changes[p]++
			currentLuma := int(frame[i]) + int(frame[i+1]) + int(frame[i+2])
			previousLuma := int(a.previous[i]) + int(a.previous[i+1]) + int(a.previous[i+2])
			sign := int8(1)
			if currentLuma < previousLuma {
				sign = -1
			}
			if a.lastSign[p] != 0 && sign != a.lastSign[p] {
				a.reversals[p]++
			}
			a.lastSign[p] = sign
			activity++
		}
	}
	if activity > a.peakActivity {
		a.peakActivity, a.peakTime = activity, time
	}
	copy(a.previous, frame)
	a.frames++
	a.lastTime = time
	return nil
}

func (a *Accumulator) Image() (*image.RGBA, Stats, error) {
	if a.frames < 2 {
		return nil, Stats{}, output.New("selected interval needs at least two decoded frames", output.FixableByAgent).
			WithHint("select a longer interval or use a video with a readable frame rate")
	}
	scale := percentile99(a.magnitude)
	img := image.NewRGBA(image.Rect(0, 0, a.width, a.height))
	active := 0
	for p, magnitude := range a.magnitude {
		if magnitude == 0 {
			continue
		}
		active++
		// Square-root mapping protects low-amplitude but meaningful UI motion.
		red := uint8(math.Round(255 * math.Sqrt(math.Min(1, magnitude/scale))))
		meanTime := a.weightedTime[p] / magnitude
		// Time is normalized over observed decoded frames, which preserves an
		// interpretable early-to-late gradient even when end is omitted.
		green := uint8(math.Round(255 * math.Max(0, math.Min(1, meanTime/a.lastTime))))
		frequency := float64(a.changes[p]+a.reversals[p]) / float64(a.frames-1)
		blue := uint8(math.Round(255 * math.Sqrt(math.Min(1, frequency))))
		x, y := p%a.width, p/a.width
		img.SetRGBA(x, y, color.RGBA{R: red, G: green, B: blue, A: 255})
	}
	return img, Stats{Frames: a.frames, Coverage: float64(active) / float64(len(a.magnitude)), PeakTime: a.peakTime}, nil
}

func percentile99(values []float64) float64 {
	kept := make([]float64, 0, len(values))
	for _, v := range values {
		if v > 0 {
			kept = append(kept, v)
		}
	}
	if len(kept) == 0 {
		return 1
	}
	sort.Float64s(kept)
	return kept[int(math.Ceil(float64(len(kept))*0.99))-1]
}
