package cli

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shhac/agent-motion/internal/fixture"
)

// run executes the CLI against a synthetic video and returns decoded stdout.
func run(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	root := newRoot("test", fixture.Reference().Decoder())
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
	root := newRoot("test", fixture.Reference().Decoder())
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

// --during exists because working out a step size by hand for every event was
// the main source of manual iteration for agents using the tool.
func TestDuringSamplesAWindowEvenly(t *testing.T) {
	got, err := run(t, "sheet", "ref.mp4", "--during", "9:11", "--count", "5",
		"--quick", "-o", filepath.Join(t.TempDir(), "s.png"))
	if err != nil {
		t.Fatal(err)
	}
	tiles, _ := got["tiles"].([]any)
	if len(tiles) != 5 {
		t.Fatalf("got %d tiles, want the 5 asked for", len(tiles))
	}
	want := []float64{9, 9.5, 10, 10.5, 11}
	for i, tile := range tiles {
		at := tile.(map[string]any)["time_seconds"].(float64)
		if at != want[i] {
			t.Errorf("tile %d at %v, want %v — the window should be covered end to end", i+1, at, want[i])
		}
	}
}

func TestDuringWorksOnFrames(t *testing.T) {
	dir := t.TempDir()
	got, err := run(t, "frames", "ref.mp4", "--during", "2-5", "--count", "4", "--dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	frames, _ := got["frames"].([]any)
	if len(frames) != 4 {
		t.Fatalf("got %d frames, want 4", len(frames))
	}
}

func TestDuringRejectsNonsense(t *testing.T) {
	cases := map[string][]string{
		"no separator":  {"sheet", "ref.mp4", "--during", "13.07"},
		"reversed":      {"sheet", "ref.mp4", "--during", "5:2"},
		"with --at too": {"sheet", "ref.mp4", "--during", "2:5", "--at", "3"},
	}
	for name, args := range cases {
		if _, err := run(t, args...); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// A result with nothing in it must still carry the fields that say so. An
// absent `events` key is the worst form of "no events read as nothing
// happened": a caller indexing it does not get an empty list, it gets a crash.
// Found by a real recording of a static page, which broke the very script
// reading it.
func TestAnEmptyResultStillCarriesTheContract(t *testing.T) {
	got, err := run(t, "timeline", "ref.mp4", "--threshold", "255")
	if err != nil {
		t.Fatal(err)
	}
	events, ok := got["events"]
	if !ok {
		t.Fatal("events key is absent; a caller cannot tell an empty result from a malformed one")
	}
	if list, isList := events.([]any); !isList || len(list) != 0 {
		t.Errorf("events = %v, want an empty list", events)
	}
	for _, key := range []string{"limits", "suitability", "narrative", "analysis"} {
		if got[key] == nil {
			t.Errorf("%q is missing from a result that found nothing, which is when it matters most", key)
		}
	}
	if narrative, _ := got["narrative"].(string); !strings.Contains(narrative, "Nothing") {
		t.Errorf("narrative should say plainly that nothing changed: %q", narrative)
	}
}

// Optional coordinates are slices, not fixed-size arrays: omitempty has no
// effect on an array, so an event that never moved would report moving nowhere.
func TestNonShiftEventsCarryNoDisplacement(t *testing.T) {
	got, err := run(t, "timeline", "ref.mp4")
	if err != nil {
		t.Fatal(err)
	}
	events, _ := got["events"].([]any)
	if len(events) == 0 {
		t.Fatal("expected events")
	}
	for _, raw := range events {
		e, _ := raw.(map[string]any)
		if e["kind"] == "shift" {
			continue
		}
		if _, present := e["moved_by_pixels"]; present {
			t.Errorf("a %v event reports moved_by_pixels: %v", e["kind"], e["moved_by_pixels"])
		}
	}
}

// A list command defaults to NDJSON, so a long answer can be filtered a line
// at a time without parsing the whole of it.
func TestActivityDefaultsToNDJSON(t *testing.T) {
	root := newRoot("test", fixture.Player().Decoder())
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"activity", "player.mp4"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	out := stdout.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected records and meta lines, got %q", out)
	}
	for i, line := range lines {
		var one map[string]any
		if err := json.Unmarshal([]byte(line), &one); err != nil {
			t.Fatalf("line %d is not one JSON object: %v", i, err)
		}
	}
	for _, key := range []string{"frame_wide", "grid", "limits", "suitability"} {
		if !strings.Contains(out, `"`+key+`"`) {
			t.Errorf("meta line %q is missing; a caller must not read an empty list as nothing happening", key)
		}
	}
}

// Asking for lines and getting one enormous line answers the letter of the
// format and none of the point of it: the reason to ask is to filter.
func TestTimelineJSONLIsOneLinePerEvent(t *testing.T) {
	root := newRoot("test", fixture.Player().Decoder())
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"timeline", "player.mp4", "--format", "jsonl"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	events, meta := 0, map[string]bool{}
	for i, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		var one map[string]any
		if err := json.Unmarshal([]byte(line), &one); err != nil {
			t.Fatalf("line %d is not one JSON object: %v", i, err)
		}
		if _, ok := one["kind"]; ok {
			events++
			continue
		}
		for k := range one {
			meta[k] = true
		}
	}
	if events < 2 {
		t.Errorf("got %d event records; the player fixture has several", events)
	}
	// The context that stops a reader over-reading the list must survive the
	// change of format.
	for _, key := range []string{"limits", "suitability", "narrative", "analysis"} {
		if !meta[key] {
			t.Errorf("meta line %q missing from the jsonl rendering", key)
		}
	}
}
