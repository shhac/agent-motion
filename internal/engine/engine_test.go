package engine_test

import (
	"context"
	"errors"
	"image"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shhac/agent-motion/internal/engine"
	"github.com/shhac/agent-motion/internal/fixture"
	"github.com/shhac/agent-motion/internal/video"
	output "github.com/shhac/lib-agent-output"
)

func referenceDecoder() (*video.Fake, fixture.Scenario) {
	s := fixture.Reference()
	return s.Decoder(), s
}

func TestAnalyseDescribesTheWholeVideo(t *testing.T) {
	dec, s := referenceDecoder()
	a, err := engine.New(dec).Analyse(context.Background(), engine.AnalyseOptions{Path: "ref.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Narrative == "" || len(a.Events) == 0 {
		t.Fatalf("empty analysis: %+v", a)
	}
	if a.Params.FramesAnalysed != s.Frames {
		t.Errorf("analysed %d frames, want %d", a.Params.FramesAnalysed, s.Frames)
	}
	if a.Params.Width != engine.DefaultAnalysisWidth {
		t.Errorf("analysis width = %d, want the %d default", a.Params.Width, engine.DefaultAnalysisWidth)
	}
	if len(a.Limits) == 0 {
		t.Error("results must state their limits so a caller does not over-read them")
	}
	if len(a.NextSteps) == 0 {
		t.Error("results should propose a runnable next command")
	}
	if a.Activity != nil {
		t.Error("the numeric series should be omitted unless asked for")
	}
	if a.Sparkline == "" {
		t.Error("the sparkline should always be present")
	}
}

func TestAnalyseHonoursTheSelectedInterval(t *testing.T) {
	dec, _ := referenceDecoder()
	a, err := engine.New(dec).Analyse(context.Background(), engine.AnalyseOptions{
		Path: "ref.mp4", Start: 8, End: 13,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range a.Events {
		if e.End < 7.5 || e.Start > 13.5 {
			t.Errorf("event %+v is outside the requested 8-13s interval", e)
		}
	}
	req := dec.Requests[len(dec.Requests)-1]
	if req.Start != 8 || req.End != 13 {
		t.Errorf("decoder asked for %.2f-%.2f, want 8-13", req.Start, req.End)
	}
}

func TestNativeAnalysesAtSourceResolution(t *testing.T) {
	dec, s := referenceDecoder()
	a, err := engine.New(dec).Analyse(context.Background(), engine.AnalyseOptions{Path: "ref.mp4", Native: true})
	if err != nil {
		t.Fatal(err)
	}
	if a.Params.Width != s.Width || a.Params.Height != s.Height {
		t.Errorf("analysed at %dx%d, want the source %dx%d", a.Params.Width, a.Params.Height, s.Width, s.Height)
	}
}

func TestAnalyseRejectsAStartPastTheEnd(t *testing.T) {
	dec, _ := referenceDecoder()
	_, err := engine.New(dec).Analyse(context.Background(), engine.AnalyseOptions{Path: "ref.mp4", Start: 99})
	assertFixableBy(t, err, output.FixableByAgent)
}

func TestAnalyseSurfacesProbeFailure(t *testing.T) {
	dec, _ := referenceDecoder()
	dec.ProbeErr = output.New("no such file", output.FixableByHuman)
	_, err := engine.New(dec).Analyse(context.Background(), engine.AnalyseOptions{Path: "missing.mp4"})
	assertFixableBy(t, err, output.FixableByHuman)
}

func TestAnalyseRejectsAnIntervalTooShortToDifference(t *testing.T) {
	dec, _ := referenceDecoder()
	_, err := engine.New(dec).Analyse(context.Background(), engine.AnalyseOptions{
		Path: "ref.mp4", Start: 1, End: 1.01,
	})
	assertFixableBy(t, err, output.FixableByAgent)
}

func TestWriteProjectionProducesAnOpaqueImage(t *testing.T) {
	dec, _ := referenceDecoder()
	e := engine.New(dec)
	a, err := e.Analyse(context.Background(), engine.AnalyseOptions{Path: "ref.mp4", Native: true})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "out.png")
	result, err := e.WriteProjection(a, path, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output != path {
		t.Errorf("output = %q, want %q", result.Output, path)
	}
	for _, channel := range []string{"red", "green", "blue", "black"} {
		if result.Encoding[channel] == "" {
			t.Errorf("encoding is the image's API and must explain %q", channel)
		}
	}
	if len(result.Excluded) == 0 {
		t.Error("the reference video contains cuts, which must be reported as excluded")
	}
}

func TestFramesRejectsTimestampsPastTheEnd(t *testing.T) {
	dec, _ := referenceDecoder()
	_, err := engine.New(dec).Frames(context.Background(), engine.FramesOptions{
		Path: "ref.mp4", At: []float64{1, 500}, Dir: t.TempDir(),
	})
	assertFixableBy(t, err, output.FixableByAgent)
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should name the offending timestamp: %v", err)
	}
}

func TestFramesRequiresTimestamps(t *testing.T) {
	dec, _ := referenceDecoder()
	_, err := engine.New(dec).Frames(context.Background(), engine.FramesOptions{Path: "ref.mp4", Dir: t.TempDir()})
	assertFixableBy(t, err, output.FixableByAgent)
}

func TestFramesWritesOnePNGPerTimestamp(t *testing.T) {
	dec, _ := referenceDecoder()
	dir := t.TempDir()
	set, err := engine.New(dec).Frames(context.Background(), engine.FramesOptions{
		Path: "ref.mp4", At: []float64{3.5, 1.25, 3.5}, Dir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Frames) != 2 {
		t.Fatalf("got %d frames, want 2 after de-duplicating", len(set.Frames))
	}
	if set.Frames[0].Time != 1.25 {
		t.Errorf("frames should come back in time order, got %v", set.Frames)
	}
	if got := dec.Stills; len(got) != 2 {
		t.Errorf("decoder asked for %v, want two stills", got)
	}
}

func TestSheetChoosesItsOwnMomentsWhenNoneGiven(t *testing.T) {
	dec, _ := referenceDecoder()
	result, err := engine.New(dec).Sheet(context.Background(), engine.SheetOptions{
		Path: "ref.mp4", Count: 8, Width: 160,
		Output:  filepath.Join(t.TempDir(), "sheet.png"),
		Analyse: engine.AnalyseOptions{Path: "ref.mp4"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tiles) != 8 {
		t.Errorf("got %d tiles, want the 8 requested", len(result.Tiles))
	}
	if result.Analysis == nil {
		t.Error("a self-chosen sheet should return the analysis that chose the moments")
	}
	labelled := 0
	for _, tile := range result.Tiles {
		if tile.Event != "" {
			labelled++
		}
	}
	if labelled == 0 {
		t.Error("tiles landing inside an event should say which event")
	}
}

// Given timestamps, the sheet still analyses: an unlabelled tile makes the
// reader match times to events by hand, and a blank-looking tile is
// indistinguishable from a broken one. What it drops is the bulky analysis
// blob, which a caller who passed --at has usually already got.
func TestSheetWithTimestampsStillLabelsTiles(t *testing.T) {
	dec, _ := referenceDecoder()
	result, err := engine.New(dec).Sheet(context.Background(), engine.SheetOptions{
		Path: "ref.mp4", At: []float64{3.5, 9.5, 15}, Width: 160,
		Output:  filepath.Join(t.TempDir(), "sheet.png"),
		Analyse: engine.AnalyseOptions{Path: "ref.mp4"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Analysis != nil {
		t.Error("with --at the full analysis blob should be left out")
	}
	if result.Narrative == "" || result.Suitability.Verdict == "" {
		t.Error("a sheet must still say what kind of recording this is")
	}
	labelled := 0
	for _, tile := range result.Tiles {
		if tile.Event != "" {
			labelled++
		}
	}
	if labelled == 0 {
		t.Errorf("tiles inside an event should name it, got %+v", result.Tiles)
	}
}

func TestSheetQuickSkipsTheAnalysisPass(t *testing.T) {
	dec, _ := referenceDecoder()
	result, err := engine.New(dec).Sheet(context.Background(), engine.SheetOptions{
		Path: "ref.mp4", At: []float64{1, 2, 3}, Width: 160, Quick: true,
		Output: filepath.Join(t.TempDir(), "sheet.png"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(dec.Requests) != 0 {
		t.Errorf("--quick should decode no interval, got %v", dec.Requests)
	}
	if result.Analysis != nil || result.Narrative != "" {
		t.Error("--quick should return no analysis at all")
	}
}

func TestSheetCropsToARegion(t *testing.T) {
	dec, _ := referenceDecoder()
	result, err := engine.New(dec).Sheet(context.Background(), engine.SheetOptions{
		Path: "ref.mp4", At: []float64{7}, Width: 240, Quick: true,
		Region: engine.Region{Box: image.Rect(500, 300, 560, 324), Pad: 10},
		Output: filepath.Join(t.TempDir(), "sheet.png"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := [4]int{490, 290, 570, 334}; result.Region != want {
		t.Errorf("region = %v, want the padded box %v", result.Region, want)
	}
}

func TestRegionPaddingStaysInsideTheFrame(t *testing.T) {
	info := fixture.Reference().Info()
	got := engine.Region{Box: image.Rect(0, 0, 40, 40), Pad: 100}.Rect(info)
	if got.Min.X != 0 || got.Min.Y != 0 {
		t.Errorf("padding must not go negative, got %v", got)
	}
	if got.Max.X > info.Width || got.Max.Y > info.Height {
		t.Errorf("padding must not exceed the frame, got %v", got)
	}
	if empty := (engine.Region{}); !empty.Rect(info).Empty() {
		t.Error("no region means no crop")
	}
}

func TestInspectDoesNotDecode(t *testing.T) {
	dec, s := referenceDecoder()
	result, err := engine.New(dec).Inspect(context.Background(), "ref.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if result.Source.Width != s.Width || result.Source.FPS != s.FPS {
		t.Errorf("source = %+v, want the probed facts", result.Source)
	}
	if len(dec.Requests) != 0 {
		t.Error("inspect must not decode frames")
	}
	if len(result.NextSteps) == 0 {
		t.Error("inspect should point at the next command")
	}
}

func assertFixableBy(t *testing.T, err error, want output.FixableBy) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error fixable by %v", want)
	}
	var structured *output.Error
	if !errors.As(err, &structured) {
		t.Fatalf("error %v is not structured; agents rely on fixable_by", err)
	}
	if structured.FixableBy != want {
		t.Errorf("fixable_by = %v, want %v", structured.FixableBy, want)
	}
}
