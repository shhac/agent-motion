package motion

// Every user-facing sentence describing one event. It is a file rather than a
// convention because three other files call into it purely for wording —
// resolve.go when a shift or an overlay is settled, segment.go when a
// whole-frame change is, describe.go when an event is first classified — and a
// change of phrasing should be one edit in one place, not four across the
// package. Classification decides what a thing is; this decides how to say it.

import (
	"fmt"
	"math"
	"strings"
)

// summarise owns every user-facing sentence describing an event, so a change of
// wording is one edit rather than four.
func summarise(e Event, opt TimelineOptions) string {
	size := regionSize(e.Region)
	duration := e.End - e.Start

	switch e.Kind {
	case KindGradual:
		if e.RegionArea > frameWideArea {
			return fmt.Sprintf("Most of the frame (%s) differs from itself %.1fs earlier, throughout %s to %s. That is what continuous motion looks like over the slow window, not a single gradual change.",
				size, opt.DriftSeconds, clock(e.Start), clock(e.End))
		}
		return fmt.Sprintf("Gradual change from %s to %s in the %s (%s). Too slow to clear the threshold between adjacent frames; only visible over the %.1fs drift window.",
			clock(e.Start), clock(e.End), e.Position, size, opt.DriftSeconds)
	case KindShift:
		return fmt.Sprintf("Content in the %s (%s) moved %s at %s%s. The same content is in a new place rather than having appeared or changed, which on a page is a layout shift. Score %.4f — the share of the frame affected times how far it went, not Chrome's CLS.",
			e.Position, size, movement(e.MovedBy), clock(e.Start), settled(e.Persists), e.ShiftScore)
	case KindStep:
		return fmt.Sprintf("One-off change at %s in the %s (%s) that is still there afterwards — something appeared, vanished, or switched state.",
			clock(e.Start), e.Position, size)
	case KindBlip:
		if duration > 0 {
			return fmt.Sprintf("Change at %s in the %s (%s) that reverts %.0f ms later — the region ends up as it started.",
				clock(e.Start), e.Position, size, duration*1000)
		}
		return fmt.Sprintf("Brief change at %s in the %s (%s) that reverts immediately.",
			clock(e.Start), e.Position, size)
	case KindFlicker:
		return fmt.Sprintf("Repeated toggling from %s to %s in the %s (%s), about %.1f changes per second over %.2fs.%s",
			clock(e.Start), clock(e.End), e.Position, size, e.ChangesPerSecond, duration, animating(e, duration))
	case KindMotion:
		text := fmt.Sprintf("Movement from %s to %s in the %s (%s); the active area travels %s across about %d px. The region is the whole path swept, not the size of the thing moving.",
			clock(e.Start), clock(e.End), e.Position, size, e.Direction, e.TravelPixels)
		if e.JumpPixels > 0 {
			text += fmt.Sprintf(" It does not move smoothly: at %s it jumps about %d px backwards before carrying on.",
				clock(e.JumpSeconds), e.JumpPixels)
		}
		return text
	default:
		if e.RegionArea > frameWideArea {
			return fmt.Sprintf("Most of the frame (%s) is changing continuously from %s to %s. This is whole-frame motion rather than one localised event; its start and end are where activity crossed the noise floor, not real boundaries.",
				size, clock(e.Start), clock(e.End))
		}
		return fmt.Sprintf("Sustained activity from %s to %s in the %s (%s), averaging %.2f%% of the frame changing per step.%s",
			clock(e.Start), clock(e.End), e.Position, size, e.MeanChanged*100, animating(e, duration))
	}
}

// animating names the shape when an event looks like ongoing decoration, so a
// long stretch of activity is not mistaken for a long stretch of trouble.
func animating(e Event, duration float64) string {
	if !e.Continuous {
		return ""
	}
	return fmt.Sprintf(" It runs steadily for %.1fs in one small fixed place, which is the shape of something animating continuously rather than a fault — but the tool cannot tell a marquee from a stuck render, so look before deciding.",
		duration)
}

func wholeFrameSummary(e Event) string {
	if e.Kind == KindFlash {
		return fmt.Sprintf("Whole-frame flash at %s lasting about %.0f ms; the picture then returns to what it was.%s",
			clock(e.Start), math.Max(1, (e.End-e.Start)*1000), shading(e))
	}
	return fmt.Sprintf("Hard cut at %s: %.0f%% of the frame changes in a single transition and stays changed.%s",
		clock(e.Start), e.PeakChanged*100, shading(e))
}

// shading says whether a whole-frame change was the picture changing or only
// its brightness, which is the difference between a new screen and the same
// screen re-shaded.
//
// The map can invert as well as scale, and the two want different sentences: a
// scrim is something laid over the content, an inversion is the content itself
// re-coloured. Both leave the picture underneath recoverable, which is the
// question being answered; only one of them has anything on top.
func shading(e Event) string {
	if e.ShadeFit == 0 {
		return ""
	}
	if !e.Uniform {
		return fmt.Sprintf(" Only %.0f%% of it follows a single brightness map, so the content itself changed rather than just its brightness.", e.ShadeFit*100)
	}
	if e.ShadeScale < 0 {
		return fmt.Sprintf(" %.0f%% of the frame moved through the same brightness map, inverted, so what is underneath is the same content with light and dark exchanged rather than a new screen.",
			e.ShadeFit*100)
	}
	rest := ""
	if e.ShadeFit < 0.97 {
		rest = fmt.Sprintf(" The remaining %.0f%% is new content on top of it, which is what a dialog over a dimmed page looks like.",
			(1-e.ShadeFit)*100)
	}
	return fmt.Sprintf(" %.0f%% of the frame moved through the same brightness map, scaled to %.0f%%, so what is underneath is unchanged — something translucent laid over it rather than a new screen.%s",
		e.ShadeFit*100, e.ShadeScale*100, rest)
}

func compass(dx, dy float64) string {
	horizontal, vertical := "", ""
	if dx > 0 {
		horizontal = "left to right"
	} else if dx < 0 {
		horizontal = "right to left"
	}
	if dy > 0 {
		vertical = "top to bottom"
	} else if dy < 0 {
		vertical = "bottom to top"
	}
	switch {
	case math.Abs(dx) >= 2*math.Abs(dy):
		return horizontal
	case math.Abs(dy) >= 2*math.Abs(dx):
		return vertical
	}
	return join(horizontal, vertical, " and ")
}

func join(a, b, sep string) string {
	switch {
	case a != "" && b != "":
		return a + sep + b
	case a != "":
		return a
	default:
		return b
	}
}

// movement puts a displacement into words, because "moved down 40 px" is
// actionable in a way that [0, 40] is not.
func movement(by []int) string {
	if len(by) < 2 {
		return "nowhere"
	}
	parts := make([]string, 0, 2)
	if by[1] != 0 {
		parts = append(parts, fmt.Sprintf("down %d px", by[1]))
		if by[1] < 0 {
			parts[len(parts)-1] = fmt.Sprintf("up %d px", -by[1])
		}
	}
	if by[0] != 0 {
		word := fmt.Sprintf("right %d px", by[0])
		if by[0] < 0 {
			word = fmt.Sprintf("left %d px", -by[0])
		}
		parts = append(parts, word)
	}
	if len(parts) == 0 {
		return "nowhere"
	}
	return strings.Join(parts, " and ")
}

func settled(persists *bool) string {
	if reverted(persists) {
		return ", and moves back"
	}
	return ", and stays there"
}
