package fixture

import "image"

// Player is the generalisation test: a video-player UI whose progress bar moves
// continuously for the whole recording, with three faults hiding behind it. A
// constantly advancing element is the common real-world case that a naive
// analysis reports as one enormous event covering everything.
func Player() Scenario {
	return Scenario{
		Name: "player", draw: drawPlayer,
		Width: 640, Height: 360, FPS: 30, Frames: 660,
		Events: []Event{
			{
				Name: "progress-bar", Kind: "baseline", Start: 0, End: 22,
				Region:      image.Rect(20, 316, 620, 324),
				Description: "A progress bar advances left to right for the whole recording.",
			},
			{
				Name: "progress-regression", Kind: "jump", Start: 8, End: 8.04,
				Region:      image.Rect(155, 316, 240, 324),
				Description: "The progress bar jumps backwards to where it was at 5s, then carries on.",
			},
			{
				Name: "thumbnail-flicker", Kind: "flicker", Start: 13, End: 13.4,
				Region:      image.Rect(500, 40, 580, 100),
				Description: "A thumbnail toggles on and off every 2 frames for 0.4s.",
			},
			{
				Name: "caption-shift", Kind: "shift", Start: 17, End: 22,
				Region:      image.Rect(60, 240, 400, 276),
				Description: "A caption block drops 6px and stays there.",
			},
		},
	}
}

var (
	playerStage    = rgb{0x10, 0x12, 0x16}
	playerControls = rgb{0x1c, 0x20, 0x28}
	playerTrack    = rgb{0x33, 0x39, 0x45}
	playerFill     = rgb{0x4a, 0x9e, 0xff}
	playerCaption  = rgb{0x8a, 0x93, 0xa5}
	playerThumb    = rgb{0xd8, 0xdd, 0xe6}
)

// progressAt is the played fraction. It advances linearly except for one jump
// backwards, which is the fault the scenario is built around.
func progressAt(t float64) float64 {
	if t >= 8 {
		t -= 3
	}
	return clamp(t / 22)
}

func drawPlayer(s Scenario, dst []byte, index int) {
	t := float64(index) / s.FPS

	fill(dst, s.Width, image.Rect(0, 0, s.Width, s.Height), playerStage)
	fill(dst, s.Width, image.Rect(0, 300, s.Width, s.Height), playerControls)
	fill(dst, s.Width, image.Rect(20, 316, 620, 324), playerTrack)
	fill(dst, s.Width, image.Rect(20, 316, 20+int(progressAt(t)*600), 324), playerFill)

	// The caption block drops six pixels at 17s and stays down.
	top := 240
	if t >= 17 {
		top = 246
	}
	fill(dst, s.Width, image.Rect(60, top, 400, top+30), playerCaption)

	if within(t, 13, 13.4) && (index/2)%2 == 0 {
		fill(dst, s.Width, image.Rect(500, 40, 580, 100), playerThumb)
	}
}
