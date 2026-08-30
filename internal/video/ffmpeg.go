package video

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"

	output "github.com/shhac/lib-agent-output"
)

// FFmpeg decodes with locally installed FFmpeg and FFprobe executables.
type FFmpeg struct {
	FFmpegPath  string
	FFprobePath string
}

// NewFFmpeg returns a decoder using the given executable names or paths.
func NewFFmpeg(ffmpegPath, ffprobePath string) *FFmpeg {
	return &FFmpeg{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
}

// Available reports a structured error when either executable is missing.
func (f *FFmpeg) Available() error {
	for _, dep := range []struct{ name, path string }{
		{"ffmpeg", f.FFmpegPath}, {"ffprobe", f.FFprobePath},
	} {
		if _, err := exec.LookPath(dep.path); err != nil {
			return output.New(fmt.Sprintf("%s executable %q was not found", dep.name, dep.path), output.FixableByHuman).
				WithHint("install FFmpeg (macOS: brew install ffmpeg) or pass --" + dep.name + " /path/to/" + dep.name)
		}
	}
	return nil
}

type probeResponse struct {
	Streams []stream `json:"streams"`
	Format  struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

type stream struct {
	CodecType    string     `json:"codec_type"`
	CodecName    string     `json:"codec_name"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	AvgFrameRate string     `json:"avg_frame_rate"`
	PixFmt       string     `json:"pix_fmt"`
	NBFrames     string     `json:"nb_frames"`
	SideData     []sideData `json:"side_data_list"`
}

type sideData struct {
	Rotation int `json:"rotation"`
}

// Probe reads stream metadata without decoding pixels.
func (f *FFmpeg) Probe(ctx context.Context, path string) (Info, error) {
	if err := f.Available(); err != nil {
		return Info{}, err
	}
	cmd := exec.CommandContext(ctx, f.FFprobePath, "-v", "error",
		"-show_entries", "stream=codec_type,codec_name,width,height,avg_frame_rate,pix_fmt,nb_frames:stream_side_data=rotation:format=duration,bit_rate",
		"-of", "json", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		return Info{}, output.Wrap(fmt.Errorf("ffprobe %q: %w", path, err), output.FixableByAgent).
			WithHint(hintOrDefault(stderr.String(), "check the path exists and FFprobe can read the file"))
	}
	var response probeResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return Info{}, output.Wrap(err, output.FixableByRetry).WithHint("FFprobe returned unparseable JSON")
	}
	return infoFrom(response)
}

// infoFrom maps an FFprobe response onto Info. It is separated from running the
// process because the mapping rules — first video stream wins, audio anywhere
// counts, a frame rate must be positive — are the part worth testing, and a test
// for them should not need FFprobe installed.
func infoFrom(response probeResponse) (Info, error) {
	info := Info{}
	found := false
	for _, s := range response.Streams {
		if s.CodecType == "audio" {
			info.HasAudio = true
			continue
		}
		if s.CodecType != "video" || found {
			continue
		}
		found = true
		info.Width, info.Height, info.Codec, info.PixelFormat = s.Width, s.Height, s.CodecName, s.PixFmt
		info.FPS, _ = ratio(s.AvgFrameRate)
		info.NBFrames, _ = strconv.Atoi(s.NBFrames)
		for _, sd := range s.SideData {
			if sd.Rotation != 0 {
				info.Rotation = sd.Rotation
			}
		}
	}
	if !found || info.Width <= 0 || info.Height <= 0 {
		return Info{}, output.New("no readable video stream in the source", output.FixableByAgent).
			WithHint("provide a file with a decodable video stream")
	}
	if info.FPS <= 0 {
		return Info{}, output.New("could not determine a positive source frame rate", output.FixableByAgent).
			WithHint("re-encode the source with a constant frame rate")
	}
	info.Duration, _ = strconv.ParseFloat(response.Format.Duration, 64)
	info.BitRate, _ = strconv.ParseInt(response.Format.BitRate, 10, 64)
	return info, nil
}

// Decode streams rgb24 frames for the requested interval. Frame.Pix is reused
// between callbacks.
func (f *FFmpeg) Decode(ctx context.Context, req Request, fn func(Frame) error) error {
	if req.Width <= 0 || req.Height <= 0 || req.FPS <= 0 {
		return output.New("decode request needs explicit frame dimensions and rate", output.FixableByRetry)
	}
	cmd := exec.CommandContext(ctx, f.FFmpegPath, decodeArgs(req)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return output.Wrap(err, output.FixableByRetry)
	}
	if err := cmd.Start(); err != nil {
		return output.Wrap(err, output.FixableByRetry)
	}
	frame := Frame{Time: req.Start, Pix: make([]byte, req.Width*req.Height*3)}
	for {
		if _, err := io.ReadFull(stdout, frame.Pix); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			_ = cmd.Wait()
			return output.Wrap(err, output.FixableByRetry)
		}
		if err := fn(frame); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}
		frame.Index++
		frame.Time = req.Start + float64(frame.Index)/req.FPS
	}
	if err := cmd.Wait(); err != nil {
		return output.Wrap(fmt.Errorf("ffmpeg decode: %w", err), output.FixableByAgent).
			WithHint(hintOrDefault(stderr.String(), "FFmpeg could not decode the requested interval"))
	}
	return nil
}

// decodeArgs builds the raw-video command line. Order matters and is part of
// the contract: -ss before -i is a fast seek, -t is a duration rather than an
// end timestamp, and the rawvideo/rgb24 tail is what makes output deterministic.
func decodeArgs(req Request) []string {
	args := []string{"-v", "error"}
	if req.Start > 0 {
		args = append(args, "-ss", seconds(req.Start))
	}
	args = append(args, "-i", req.Path)
	if req.End > req.Start {
		args = append(args, "-t", seconds(req.End-req.Start))
	}
	return append(args, "-an", "-sn", "-dn", "-vf", filters(req),
		"-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
}

// stillArgs builds the single-frame command line. Cropping happens before
// scaling, so a small region asked for at a given width comes back magnified
// rather than shrunk into the corner of a full frame.
func stillArgs(path string, still Still) []string {
	args := []string{"-v", "error"}
	if still.At > 0 {
		args = append(args, "-ss", seconds(still.At))
	}
	args = append(args, "-i", path, "-frames:v", "1", "-an", "-sn", "-dn")
	if filter := stillFilter(still); filter != "" {
		args = append(args, "-vf", filter)
	}
	return append(args, "-f", "image2", "-c:v", "png", "pipe:1")
}

func stillFilter(still Still) string {
	var stages []string
	if r := still.Crop; !r.Empty() {
		stages = append(stages, fmt.Sprintf("crop=%d:%d:%d:%d", r.Dx(), r.Dy(), r.Min.X, r.Min.Y))
	}
	if still.Width > 0 {
		stages = append(stages, fmt.Sprintf("scale=%d:-2", still.Width))
	}
	return strings.Join(stages, ",")
}

// filters pins both the frame rate and the frame size so the raw stream has a
// known shape and every frame index maps to a known timestamp.
func filters(req Request) string {
	return fmt.Sprintf("fps=%s,scale=%d:%d", seconds(req.FPS), req.Width, req.Height)
}

// Still decodes a single frame and returns it as PNG bytes.
func (f *FFmpeg) Still(ctx context.Context, path string, still Still) ([]byte, error) {
	cmd := exec.CommandContext(ctx, f.FFmpegPath, stillArgs(path, still)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		return nil, output.Wrap(fmt.Errorf("ffmpeg still at %.3fs: %w", still.At, err), output.FixableByAgent).
			WithHint(hintOrDefault(stderr.String(), "the timestamp may be past the end of the video"))
	}
	if len(raw) == 0 {
		return nil, output.New(fmt.Sprintf("no frame decoded at %.3fs", still.At), output.FixableByAgent).
			WithHint("choose a timestamp inside the video duration")
	}
	return raw, nil
}

// FitWidth returns even analysis dimensions no wider than max, preserving the
// source aspect ratio. A max of zero or wider than the source keeps native size.
func FitWidth(info Info, max int) (int, int) {
	if max <= 0 || max >= info.Width {
		return info.Width, info.Height
	}
	height := int(math.Round(float64(info.Height) * float64(max) / float64(info.Width)))
	return even(max), even(height)
}

func even(v int) int {
	if v < 2 {
		return 2
	}
	return v - v%2
}

func seconds(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func hintOrDefault(stderr, fallback string) string {
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		return trimmed
	}
	return fallback
}

func ratio(s string) (float64, error) {
	n, d, ok := strings.Cut(s, "/")
	if !ok {
		return strconv.ParseFloat(s, 64)
	}
	num, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return 0, err
	}
	den, err := strconv.ParseFloat(d, 64)
	if err != nil || den == 0 {
		return 0, fmt.Errorf("bad frame rate %q", s)
	}
	return num / den, nil
}
