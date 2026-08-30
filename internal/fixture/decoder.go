package fixture

import "github.com/shhac/agent-motion/internal/video"

// Info describes the scenario the way a decoder would report it.
func (s Scenario) Info() video.Info {
	return video.Info{
		Width: s.Width, Height: s.Height, FPS: s.FPS,
		Duration: s.Duration(), NBFrames: s.Frames,
		Codec: "h264", PixelFormat: "yuv420p",
	}
}

// Decoder returns a fake decoder that replays this scenario. Tests that need a
// variant mutate the returned value; every test that just needs a source video
// gets one from here, so the mapping between a scenario and what a decoder
// reports is written once.
func (s Scenario) Decoder() *video.Fake {
	return &video.Fake{Info: s.Info(), Render: s.Frame}
}
