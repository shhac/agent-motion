// Command genfixture renders the reference scenario to an MP4 and writes the
// matching ground-truth JSON. It exists for evaluation runs and manual
// inspection; it is not part of the shipped CLI.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/shhac/agent-motion/internal/fixture"
)

func main() {
	video := flag.String("video", "reference.mp4", "MP4 destination")
	truth := flag.String("truth", "", "Optional ground-truth JSON destination")
	ffmpegPath := flag.String("ffmpeg", "ffmpeg", "FFmpeg executable")
	crf := flag.Int("crf", 20, "x264 constant rate factor; 0 is lossless")
	name := flag.String("scenario", "reference", "Scenario to render: reference or defect")
	flag.Parse()

	s, err := scenario(*name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := render(s, *video, *ffmpegPath, *crf); err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}
	if *truth != "" {
		if err := writeTruth(s, *truth); err != nil {
			fmt.Fprintln(os.Stderr, "truth:", err)
			os.Exit(1)
		}
	}
	fmt.Printf("wrote %s (%dx%d, %.0f fps, %d frames, %.2fs)\n",
		*video, s.Width, s.Height, s.FPS, s.Frames, s.Duration())
}

func scenario(name string) (fixture.Scenario, error) {
	switch name {
	case "reference":
		return fixture.Reference(), nil
	case "defect":
		return fixture.Defect(), nil
	default:
		return fixture.Scenario{}, fmt.Errorf("unknown scenario %q", name)
	}
}

func render(s fixture.Scenario, dst, ffmpegPath string, crf int) error {
	cmd := exec.Command(ffmpegPath, "-v", "error", "-y",
		"-f", "rawvideo", "-pix_fmt", "rgb24",
		"-s", fmt.Sprintf("%dx%d", s.Width, s.Height),
		"-r", strconv.FormatFloat(s.FPS, 'f', -1, 64),
		"-i", "pipe:0",
		"-c:v", "libx264", "-preset", "veryslow", "-crf", strconv.Itoa(crf),
		"-pix_fmt", "yuv420p", dst)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	frame := make([]byte, s.Width*s.Height*3)
	for i := 0; i < s.Frames; i++ {
		s.Frame(frame, i)
		if _, err := stdin.Write(frame); err != nil {
			_ = stdin.Close()
			_ = cmd.Wait()
			return err
		}
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	return cmd.Wait()
}

type truthEvent struct {
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Start       float64 `json:"start_seconds"`
	End         float64 `json:"end_seconds"`
	Region      [4]int  `json:"region_xyxy"`
	Description string  `json:"description"`
}

func writeTruth(s fixture.Scenario, dst string) error {
	events := make([]truthEvent, 0, len(s.Events))
	for _, e := range s.Events {
		events = append(events, truthEvent{
			Name: e.Name, Kind: e.Kind, Start: e.Start, End: e.End,
			Region:      [4]int{e.Region.Min.X, e.Region.Min.Y, e.Region.Max.X, e.Region.Max.Y},
			Description: e.Description,
		})
	}
	body, err := json.MarshalIndent(map[string]any{
		"width": s.Width, "height": s.Height, "fps": s.FPS,
		"frames": s.Frames, "duration_seconds": s.Duration(), "events": events,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dst, append(body, '\n'), 0o644)
}
