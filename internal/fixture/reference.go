package fixture

import "image"

// Reference is the scenario used by tests and by evaluation fixtures.
func Reference() Scenario {
	return Scenario{
		Name: "reference", draw: drawReference,
		Width: 640, Height: 360, FPS: 30, Frames: 840,
		Events: []Event{
			{
				Name: "moving-dot", Kind: "motion", Start: 2, End: 5,
				Region:      image.Rect(140, 160, 632, 192),
				Description: "A 32px amber square travels left to right across the middle band.",
			},
			{
				Name: "appear-badge", Kind: "appearance", Start: 6.5, End: 28,
				Region:      image.Rect(500, 300, 560, 324),
				Description: "A green badge appears once in the lower right and then stays.",
			},
			{
				Name: "flicker-panel", Kind: "flicker", Start: 9, End: 12,
				Region:      image.Rect(300, 60, 380, 140),
				Description: "A cyan panel toggles on and off every 3 frames (5 Hz).",
			},
			{
				Name: "scene-cut", Kind: "cut", Start: 15, End: 18,
				Region:      image.Rect(0, 0, 640, 360),
				Description: "The whole frame cuts to a light alternate scene, then cuts back.",
			},
			{
				Name: "single-frame-flash", Kind: "glitch", Start: 21, End: 21.0333,
				Region:      image.Rect(0, 0, 640, 360),
				Description: "Exactly one all-white frame.",
			},
			{
				Name: "fade-region", Kind: "fade", Start: 23, End: 27,
				Region:      image.Rect(200, 200, 400, 320),
				Description: "A rectangle fades linearly from the background colour to magenta.",
			},
		},
	}
}
