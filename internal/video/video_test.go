package video

import "testing"

func TestRatioReadsFFprobeFrameRates(t *testing.T) {
	cases := map[string]float64{
		"30/1":       30,
		"30000/1001": 29.97,
		"25":         25,
	}
	for input, want := range cases {
		got, err := ratio(input)
		if err != nil {
			t.Errorf("ratio(%q): %v", input, err)
			continue
		}
		if got < want-0.01 || got > want+0.01 {
			t.Errorf("ratio(%q) = %v, want about %v", input, got, want)
		}
	}
	if _, err := ratio("30/0"); err == nil {
		t.Error("a zero denominator should be an error, not an infinite frame rate")
	}
}

func TestFitWidthKeepsAspectAndEvenDimensions(t *testing.T) {
	info := Info{Width: 1920, Height: 1080}
	w, h := FitWidth(info, 321)
	if w != 320 {
		t.Errorf("width = %d, want 320: odd sizes break yuv chroma subsampling", w)
	}
	if h != 180 {
		t.Errorf("height = %d, want 180 to preserve 16:9", h)
	}
	if w, h := FitWidth(info, 0); w != 1920 || h != 1080 {
		t.Errorf("a zero max should keep the native size, got %dx%d", w, h)
	}
	if w, h := FitWidth(info, 4000); w != 1920 || h != 1080 {
		t.Errorf("a max wider than the source should not upscale, got %dx%d", w, h)
	}
}

// TestFiltersPinRateAndSize guards determinism: both the rate and the size are
// forced, so frame index always maps to a known timestamp and a known shape.
func TestFiltersPinRateAndSize(t *testing.T) {
	got := filters(Request{Width: 320, Height: 180, FPS: 25})
	if got != "fps=25,scale=320:180" {
		t.Errorf("filters = %q", got)
	}
}

func TestAvailableReportsMissingExecutablesAsHumanFixable(t *testing.T) {
	err := NewFFmpeg("definitely-not-a-real-ffmpeg", "ffprobe").Available()
	if err == nil {
		t.Fatal("expected an error for a missing executable")
	}
	if got := err.Error(); got == "" {
		t.Error("error should name the missing executable")
	}
}

// TestInfoFromMapsProbeResponses covers the rules that decide what a source is,
// without needing FFprobe installed or a media file on disk.
func TestInfoFromMapsProbeResponses(t *testing.T) {
	video := func(w, h int, rate string) stream {
		return stream{CodecType: "video", CodecName: "h264", Width: w, Height: h, AvgFrameRate: rate}
	}

	t.Run("first video stream wins and audio is noticed", func(t *testing.T) {
		got, err := infoFrom(probeResponse{Streams: []stream{
			{CodecType: "audio"},
			video(1920, 1080, "30/1"),
			video(320, 180, "60/1"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		if got.Width != 1920 || got.FPS != 30 {
			t.Errorf("got %dx%d @%v, want the first video stream", got.Width, got.Height, got.FPS)
		}
		if !got.HasAudio {
			t.Error("an audio stream anywhere in the file should be reported")
		}
	})

	t.Run("rotation comes from side data", func(t *testing.T) {
		s := video(1080, 1920, "30/1")
		s.SideData = []sideData{{Rotation: 0}, {Rotation: -90}}
		got, err := infoFrom(probeResponse{Streams: []stream{s}})
		if err != nil {
			t.Fatal(err)
		}
		if got.Rotation != -90 {
			t.Errorf("rotation = %d, want -90", got.Rotation)
		}
	})

	t.Run("unreadable sources are agent-fixable errors", func(t *testing.T) {
		cases := map[string]probeResponse{
			"no streams at all":   {},
			"audio only":          {Streams: []stream{{CodecType: "audio"}}},
			"zero dimensions":     {Streams: []stream{video(0, 0, "30/1")}},
			"unusable frame rate": {Streams: []stream{video(640, 360, "0/0")}},
		}
		for name, response := range cases {
			if _, err := infoFrom(response); err == nil {
				t.Errorf("%s: expected an error", name)
			}
		}
	})

	t.Run("optional fields survive being absent", func(t *testing.T) {
		s := video(640, 360, "30/1")
		s.NBFrames = "N/A"
		got, err := infoFrom(probeResponse{Streams: []stream{s}})
		if err != nil {
			t.Fatalf("a missing frame count should not fail the probe: %v", err)
		}
		if got.NBFrames != 0 || got.Duration != 0 {
			t.Errorf("got %+v, want zeroes rather than garbage for absent fields", got)
		}
	})
}

// TestDecodeArgsHoldTheDeterminismContract pins the flag order that makes
// decoding reproducible: a fast seek, a duration rather than an end, and a
// pinned rate, size and pixel format.
func TestDecodeArgsHoldTheDeterminismContract(t *testing.T) {
	got := decodeArgs(Request{Path: "clip.mp4", Start: 2, End: 5, Width: 320, Height: 180, FPS: 30})
	want := []string{
		"-v", "error", "-ss", "2", "-i", "clip.mp4", "-t", "3",
		"-an", "-sn", "-dn", "-vf", "fps=30,scale=320:180",
		"-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1",
	}
	if !equal(got, want) {
		t.Errorf("decodeArgs =\n  %v\nwant\n  %v", got, want)
	}
	if i, j := indexOf(got, "-ss"), indexOf(got, "-i"); i > j {
		t.Error("-ss must precede -i, or the seek decodes the whole file")
	}
	// With no end, no duration is passed and FFmpeg runs to the end of the file.
	if plain := decodeArgs(Request{Path: "c.mp4", Width: 2, Height: 2, FPS: 1}); indexOf(plain, "-t") >= 0 {
		t.Errorf("an open-ended request should not pass -t: %v", plain)
	}
	if from := decodeArgs(Request{Path: "c.mp4", Width: 2, Height: 2, FPS: 1}); indexOf(from, "-ss") >= 0 {
		t.Errorf("a request starting at zero should not seek: %v", from)
	}
}

func TestStillArgsScaleOnlyWhenAsked(t *testing.T) {
	full := stillArgs("clip.mp4", 3.5, 0)
	if indexOf(full, "-vf") >= 0 {
		t.Errorf("width 0 means native size, so no filter: %v", full)
	}
	if indexOf(full, "-frames:v") < 0 {
		t.Errorf("a still must ask for exactly one frame: %v", full)
	}
	scaled := stillArgs("clip.mp4", 0, 320)
	if i := indexOf(scaled, "-vf"); i < 0 || scaled[i+1] != "scale=320:-2" {
		t.Errorf("scaled still args = %v", scaled)
	}
	if indexOf(scaled, "-ss") >= 0 {
		t.Errorf("a still at zero should not seek: %v", scaled)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func indexOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}
