package motion

import (
	"fmt"
	"image"
	"math"
	"strings"
)

// timescale resolves the fast/slow choice once per group. Without it the same
// "drift or changed?" decision is re-made at every site that reads a cell, and
// an edit that updates four of them and misses the fifth still compiles.
type timescale int

const (
	fast timescale = iota
	slow
	timescales
)

// groupStats is everything one pass over a group's samples establishes. Keeping
// it separate lets the description paths be exercised without an Analyzer.
type groupStats struct {
	union       image.Rectangle
	first, last image.Point
	active      int
	frames      int
	sum         float64
	peak        float64
	peakTime    float64
	peakIndex   int
	peakDrift   float64
	footprint   float64
	// path is where the activity was, one point per active transition, so a
	// discontinuity inside otherwise smooth movement can be found afterwards.
	path  []image.Point
	times []float64
}

// aggregate reduces one group's stretch of samples to its statistics.
func (a *Analyzer) aggregate(g group, floors []float64) groupStats {
	t := g.scale
	stats := groupStats{
		frames:   g.to - g.from + 1,
		peakTime: a.samples[g.from].Time, peakIndex: a.samples[g.from].Index,
	}
	seenFirst := false

	for i := g.from; i <= g.to; i++ {
		changed, drift, box, centre := a.groupSample(i, g)
		stats.sum += changed
		stats.peakDrift = math.Max(stats.peakDrift, drift)

		measure := changed
		if t == slow {
			measure = drift
		}
		if measure > stats.peak {
			stats.peak, stats.peakTime, stats.peakIndex = measure, a.samples[i].Time, a.samples[i].Index
		}
		if !a.groupActive(i, g, floors) {
			continue
		}
		stats.active++
		stats.union = stats.union.Union(box)
		stats.footprint += math.Hypot(float64(box.Dx()), float64(box.Dy()))
		stats.path = append(stats.path, centre)
		stats.times = append(stats.times, a.samples[i].Time)
		if !seenFirst {
			stats.first, seenFirst = centre, true
		}
		stats.last = centre
	}
	return stats
}

// describe builds an event from one group of cells over one stretch of samples.
func (a *Analyzer) describe(g group, floors []float64, opt TimelineOptions) (Event, bool) {
	stats := a.aggregate(g, floors)
	if stats.union.Empty() {
		return Event{}, false
	}
	e := Event{
		Start: a.samples[g.from].Time, End: a.samples[g.to].Time,
		Peak: stats.peakTime, PeakChanged: stats.peak, PeakDrift: stats.peakDrift,
		MeanChanged: stats.sum / float64(stats.frames),
		Region:      a.sourceRect(stats.union, opt),
	}
	e.RegionArea = areaFraction(e.Region, opt)
	e.Position = positionOf(e.Region, opt)
	e.Persists = a.persists(stats.union, e.Start, e.End)

	if g.scale == slow {
		return a.gradualEvent(e, opt), true
	}
	e = a.fastEvent(e, stats, opt)
	if scattered(e) {
		return Event{}, false
	}
	return e, true
}

// gradualEvent finishes a slow-timescale event. Drift reports a change one
// window after it began, so the start is shifted back by that window.
func (a *Analyzer) gradualEvent(e Event, opt TimelineOptions) Event {
	e.Kind = KindGradual
	e.Start = math.Max(0, e.Start-opt.DriftSeconds)
	e.round()
	e.Summary = summarise(e, opt)
	return e
}

func (a *Analyzer) fastEvent(e Event, stats groupStats, opt TimelineOptions) Event {
	e.Direction, e.TravelPixels = a.travel(stats, opt)
	if e.TravelPixels > 0 {
		e.JumpPixels, e.JumpSeconds = a.jump(stats, opt)
	}
	e.Kind = classify(e, stats)
	if wholeFrame(e) {
		// A "flicker across the whole frame" is not a finding, it is a
		// description of continuous motion. Drop the fields that would read as
		// one, rather than computing them and then unsetting them.
		e.Kind, e.Direction, e.TravelPixels = KindBusy, "", 0
	} else if e.Kind == KindFlicker {
		if duration := e.End - e.Start; duration > 0 {
			e.ChangesPerSecond = math.Round(float64(stats.active)/duration*10) / 10
		}
	}
	e.round()
	e.Summary = summarise(e, opt)
	return e
}

// frameWideArea is the share of the frame above which an event stops being
// something that happened somewhere and becomes something happening everywhere.
const frameWideArea = 0.6

// minDensity is the share of its own region an event must actually fill.
// Calibrated in TestDensitySeparatesChangeFromCodecNoise: a static Wikipedia
// article re-encoded to mp4 produces blips of density 0.0002, while the
// smallest real change in any fixture — a 2px card edge sliding — is 0.03.
const minDensity = 0.01

// scattered rejects a brief event whose changed pixels are spread thinly across
// its own region, which is what lossy compression looks like and what anything
// genuinely changing does not. Something real is solid within its bounds; codec
// noise is a handful of stray pixels across a large box.
//
// The test is a ratio of two fractions of the frame, so it holds at any
// resolution — a pixel count cannot, and a static page re-encoded at a higher
// resolution would sail past one.
func scattered(e Event) bool {
	if e.Kind != KindStep && e.Kind != KindBlip {
		return false // sustained and travelling events are legitimately diffuse
	}
	return e.RegionArea > 0 && e.PeakChanged/e.RegionArea < minDensity
}

// wholeFrame reports an event large enough that calling it localised would
// mislead. Steps and blips are exempt: a one-off whole-frame change is a real,
// locatable moment even though it covers everything.
func wholeFrame(e Event) bool {
	return e.RegionArea > frameWideArea && e.Kind != KindStep && e.Kind != KindBlip
}

func classify(e Event, stats groupStats) string {
	duty := float64(stats.active) / float64(stats.frames)
	// Brief means few transitions, not a short span: something that changes
	// and changes back is two transitions however far apart they are.
	brief := stats.active <= 2

	switch {
	case brief && persisted(e.Persists):
		return KindStep
	case brief:
		return KindBlip
	case stats.active >= 4 && duty < 0.75 && e.TravelPixels == 0:
		return KindFlicker
	case e.TravelPixels > 0:
		return KindMotion
	default:
		return KindBusy
	}
}

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
		return fmt.Sprintf("Repeated toggling from %s to %s in the %s (%s), about %.1f changes per second over %.2fs.",
			clock(e.Start), clock(e.End), e.Position, size, e.ChangesPerSecond, duration)
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
		return fmt.Sprintf("Sustained activity from %s to %s in the %s (%s), averaging %.2f%% of the frame changing per step.",
			clock(e.Start), clock(e.End), e.Position, size, e.MeanChanged*100)
	}
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
// its brightness, which is the difference between a new screen and a scrim over
// the old one.
func shading(e Event) string {
	if e.ShadeResidual == 0 {
		return ""
	}
	if e.Uniform {
		return fmt.Sprintf(" Every pixel moved through the same brightness map, scaled to %.0f%%, so the content underneath is unchanged — an overlay, a dim, or a theme switch rather than a new screen.",
			e.ShadeScale*100)
	}
	return " The content itself changed, not just its brightness."
}

// groupSample reduces one transition to what this group of cells saw.
func (a *Analyzer) groupSample(i int, g group) (changed, drift float64, box image.Rectangle, centre image.Point) {
	t := g.scale
	var count, driftCount int32
	var weightX, weightY, weight float64
	for _, c := range g.cells {
		cell := a.samples[i].Cells[c]
		count += cell.Count(fast)
		driftCount += cell.Count(slow)
		b := cell.Box(t)
		if b.Empty() {
			continue
		}
		box = box.Union(b)
		w := float64(cell.Count(t))
		weightX += float64(b.Min.X+b.Max.X) / 2 * w
		weightY += float64(b.Min.Y+b.Max.Y) / 2 * w
		weight += w
	}
	if weight > 0 {
		centre = image.Pt(int(weightX/weight), int(weightY/weight))
	}
	pixels := float64(a.pixels)
	return float64(count) / pixels, float64(driftCount) / pixels, box, centre
}

func (a *Analyzer) groupActive(i int, g group, floors []float64) bool {
	t := g.scale
	for _, c := range g.cells {
		if a.cellFraction(i, c, t) > floors[c] {
			return true
		}
	}
	return false
}

// travel names the direction only when the centre moves further than the
// typical per-frame footprint, so a stationary flicker is never called motion.
func (a *Analyzer) travel(stats groupStats, opt TimelineOptions) (string, int) {
	if stats.active < 2 {
		return "", 0
	}
	sx, sy := a.scale(opt)
	dx := float64(stats.last.X-stats.first.X) * sx
	dy := float64(stats.last.Y-stats.first.Y) * sy
	distance := math.Hypot(dx, dy)
	if distance < (stats.footprint/float64(stats.active))*math.Max(sx, sy) {
		return "", 0
	}
	return compass(dx, dy), int(math.Round(distance))
}

// jump finds the largest step against the overall direction of travel. Smooth
// movement has none; a progress bar snapping backwards, a scroll position
// resetting, or a carousel jumping to the start all produce one, and that
// discontinuity is usually the bug rather than the movement around it.
func (a *Analyzer) jump(stats groupStats, opt TimelineOptions) (int, float64) {
	if len(stats.path) < 3 {
		return 0, 0
	}
	sx, sy := a.scale(opt)
	netX := float64(stats.last.X-stats.first.X) * sx
	netY := float64(stats.last.Y-stats.first.Y) * sy
	length := math.Hypot(netX, netY)
	if length == 0 {
		return 0, 0
	}
	unitX, unitY := netX/length, netY/length

	worst, at := 0.0, 0.0
	for i := 1; i < len(stats.path); i++ {
		stepX := float64(stats.path[i].X-stats.path[i-1].X) * sx
		stepY := float64(stats.path[i].Y-stats.path[i-1].Y) * sy
		if along := stepX*unitX + stepY*unitY; along < worst {
			worst, at = along, stats.times[i]
		}
	}
	// A backwards step has to beat the typical per-frame footprint to be a jump
	// rather than the jitter of a bounding box around a moving thing.
	if -worst < (stats.footprint/float64(stats.active))*math.Max(sx, sy) {
		return 0, 0
	}
	return int(math.Round(-worst)), round2(at)
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

// round trims float noise so the record stays readable and diffs cleanly.
func (e *Event) round() {
	e.Start, e.End, e.Peak = round2(e.Start), round2(e.End), round2(e.Peak)
	e.PeakChanged, e.MeanChanged = round4(e.PeakChanged), round4(e.MeanChanged)
	e.PeakDrift, e.RegionArea = round4(e.PeakDrift), round4(e.RegionArea)
}

// movement puts a displacement into words, because "moved down 40 px" is
// actionable in a way that [0, 40] is not.
func movement(by [2]int) string {
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
