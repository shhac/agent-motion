package fixture

import "image"

// Defect is a scenario built the way a real bug report looks: a dashboard that
// is mostly still, a heartbeat that proves the page is alive, and three faults
// that are each easy to miss — a stall, a few frames of layout jitter, and a
// colour drift slow enough to be invisible between frames.
func Defect() Scenario {
	return Scenario{
		Name: "defect", draw: drawDefect,
		Width: 640, Height: 360, FPS: 30, Frames: 600,
		Events: []Event{
			{
				Name: "heartbeat", Kind: "baseline", Start: 0, End: 20,
				Region:      image.Rect(556, 16, 576, 36),
				Description: "A dot in the header pulses every 5 frames for the whole recording, except during the stall.",
			},
			{
				Name: "layout-jitter", Kind: "jitter", Start: 6.2, End: 6.37,
				Region:      image.Rect(200, 120, 322, 160),
				Description: "A card jumps 2px to the right for 5 frames and then jumps back.",
			},
			{
				Name: "heartbeat-stall", Kind: "freeze", Start: 11, End: 14,
				Region:      image.Rect(556, 16, 576, 36),
				Description: "The pulsing stops entirely for three seconds, then resumes. Nothing changes on screen at all during it.",
			},
			{
				Name: "status-drift", Kind: "fade", Start: 16, End: 19,
				Region:      image.Rect(450, 250, 530, 290),
				Description: "A status block drifts from green to amber over three seconds.",
			},
		},
	}
}

var (
	deskCanvas = rgb{0x18, 0x1c, 0x24}
	deskHeader = rgb{0x23, 0x29, 0x34}
	deskNav    = rgb{0x1f, 0x24, 0x2e}
	deskCard   = rgb{0x2c, 0x33, 0x40}
	pulseOn    = rgb{0x6c, 0xe0, 0xa8}
	pulseOff   = rgb{0x2a, 0x4a, 0x3c}
	statusOK   = rgb{0x4a, 0xc8, 0x7a}
	statusWarn = rgb{0xe0, 0xa8, 0x3c}
)

func drawDefect(s Scenario, dst []byte, index int) {
	t := float64(index) / s.FPS

	fill(dst, s.Width, image.Rect(0, 0, s.Width, s.Height), deskCanvas)
	fill(dst, s.Width, image.Rect(0, 0, s.Width, 52), deskHeader)
	fill(dst, s.Width, image.Rect(0, 52, 140, s.Height), deskNav)
	fill(dst, s.Width, image.Rect(360, 120, 482, 160), deskCard)
	fill(dst, s.Width, image.Rect(200, 200, 322, 240), deskCard)

	// The card that jitters, offset by two pixels for five frames.
	offset := 0
	if within(t, 6.2, 6.2+5/s.FPS) {
		offset = 2
	}
	fill(dst, s.Width, image.Rect(200+offset, 120, 322+offset, 160), deskCard)

	// The heartbeat, which stops between 11s and 14s.
	beat := pulseOff
	if !within(t, 11, 14) && (index/5)%2 == 0 {
		beat = pulseOn
	}
	fill(dst, s.Width, image.Rect(556, 16, 576, 36), beat)

	// The status block, drifting from green to amber.
	fill(dst, s.Width, image.Rect(450, 250, 530, 290),
		mix(statusOK, statusWarn, clamp((t-16)/3)))
}
