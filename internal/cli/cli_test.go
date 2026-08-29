package cli

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shhac/agent-motion/internal/fixture"
	"github.com/shhac/agent-motion/internal/video"
)

// run executes the CLI against a synthetic video and returns decoded stdout.
func run(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	s := fixture.Reference()
	dec := &video.Fake{
		Info: video.Info{
			Width: s.Width, Height: s.Height, FPS: s.FPS,
			Duration: s.Duration(), NBFrames: s.Frames, Codec: "h264",
		},
		Render:   s.Frame,
		StillPNG: stillPNG(t),
	}
	root := newRoot("test", dec)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(args)
	err := root.Execute()
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if body := stdout.Bytes(); len(body) > 0 {
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, body)
		}
	}
	return decoded, nil
}

func TestInspectPrintsSourceFacts(t *testing.T) {
	got, err := run(t, "inspect", "ref.mp4")
	if err != nil {
		t.Fatal(err)
	}
	source, ok := got["source"].(map[string]any)
	if !ok {
		t.Fatalf("no source object in %v", got)
	}
	if source["width"] != float64(640) || source["fps"] != float64(30) {
		t.Errorf("source = %v, want the probed dimensions and rate", source)
	}
}

func TestTimelineReportsEventsAndGuidance(t *testing.T) {
	got, err := run(t, "timeline", "ref.mp4")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"narrative", "events", "limits", "activity_sparkline", "analysis"} {
		if got[key] == nil {
			t.Errorf("timeline output is missing %q", key)
		}
	}
	events, _ := got["events"].([]any)
	if len(events) < 5 {
		t.Errorf("got %d events, want the reference scenario's several", len(events))
	}
	first, _ := events[0].(map[string]any)
	for _, key := range []string{"kind", "start_seconds", "summary", "region_xyxy", "position"} {
		if first[key] == nil {
			t.Errorf("event is missing %q: %v", key, first)
		}
	}
}

func TestTimelineSeriesFlagAddsNumbers(t *testing.T) {
	without, err := run(t, "timeline", "ref.mp4")
	if err != nil {
		t.Fatal(err)
	}
	with, err := run(t, "timeline", "ref.mp4", "--series")
	if err != nil {
		t.Fatal(err)
	}
	if without["activity_by_bucket"] != nil {
		t.Error("numeric buckets should be opt in")
	}
	if with["activity_by_bucket"] == nil {
		t.Error("--series should add the numeric buckets")
	}
}

func TestProjectWritesAnImageBesideTheInput(t *testing.T) {
	out := filepath.Join(t.TempDir(), "activity.png")
	got, err := run(t, "project", "ref.mp4", "-o", out)
	if err != nil {
		t.Fatal(err)
	}
	if got["output"] != out {
		t.Errorf("output = %v, want %q", got["output"], out)
	}
	if got["encoding"] == nil {
		t.Error("project must publish the channel encoding")
	}
	img := readPNG(t, out)
	if img.Bounds().Dx() != 640 {
		t.Errorf("image is %d wide, want the source width so x,y still line up", img.Bounds().Dx())
	}
	// Inactive pixels must be opaque black, or a viewer shows the page behind.
	if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0xffff {
		t.Errorf("corner alpha = %d, want fully opaque", alpha)
	}
}

func TestProjectDefaultsItsOutputPath(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "clip.mp4")
	got, err := run(t, "project", input)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "clip.temporal.png")
	if got["output"] != want {
		t.Errorf("output = %v, want %q", got["output"], want)
	}
}

func TestSheetWritesALabelledGrid(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sheet.png")
	got, err := run(t, "sheet", "ref.mp4", "--at", "1,2,3", "-o", out)
	if err != nil {
		t.Fatal(err)
	}
	tiles, _ := got["tiles"].([]any)
	if len(tiles) != 3 {
		t.Fatalf("got %d tiles, want 3", len(tiles))
	}
	if readPNG(t, out).Bounds().Dx() == 0 {
		t.Error("sheet image is empty")
	}
}

func TestFramesWritesStills(t *testing.T) {
	dir := t.TempDir()
	got, err := run(t, "frames", "ref.mp4", "--at", "2.5", "--dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	frames, _ := got["frames"].([]any)
	if len(frames) != 1 {
		t.Fatalf("got %v, want one frame", frames)
	}
	first, _ := frames[0].(map[string]any)
	readPNG(t, first["path"].(string))
}

func TestBadFlagsAreReportedAsAgentFixable(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"negative start", []string{"timeline", "ref.mp4", "--start", "-1"}, "--start"},
		{"end before start", []string{"timeline", "ref.mp4", "--start", "5", "--end", "2"}, "--end"},
		{"threshold out of range", []string{"timeline", "ref.mp4", "--threshold", "999"}, "--threshold"},
		{"unparseable timestamp", []string{"frames", "ref.mp4", "--at", "soon"}, "soon"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := run(t, tc.args...)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q should mention %q", err, tc.want)
			}
		})
	}
}

func TestYAMLFormatIsAvailable(t *testing.T) {
	s := fixture.Reference()
	dec := &video.Fake{
		Info:   video.Info{Width: s.Width, Height: s.Height, FPS: s.FPS, Duration: s.Duration(), NBFrames: s.Frames},
		Render: s.Frame, StillPNG: stillPNG(t),
	}
	root := newRoot("test", dec)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"inspect", "ref.mp4", "--format", "yaml"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "source:") {
		t.Errorf("yaml output looks wrong:\n%s", stdout.String())
	}
}

func stillPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 160, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 160; x++ {
			img.SetRGBA(x, y, color.RGBA{B: uint8(x), A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img
}
