package motion

import (
	"fmt"
	"image"
	"math"
)

// timescale resolves the fast/slow choice once per group. Without it the same
// "drift or changed?" decision is re-made at every site that reads a cell, and
// an edit that updates four of them and misses the fifth still compiles.
type timescale bool

const (
	fast timescale = false
	slow timescale = true
)

func (t timescale) count(c Cell) int32 {
	if t == slow {
		return c.Drift
	}
	return c.Changed
}

func (t timescale) box(c Cell) image.Rectangle {
	if t == slow {
		return c.DriftBox()
	}
	return c.Box()
}

func (a *Analyzer) cellMeasure(t timescale, i, c int) float64 {
	if t == fast {
		return a.cellChanged(i, c)
	}
	return a.cellDrift(i, c)
}

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
	peakDrift   float64
	footprint   float64
}

// aggregate reduces one group's stretch of samples to its statistics.
func (a *Analyzer) aggregate(g group, floors []float64) groupStats {
	t := g.timescale()
	stats := groupStats{frames: g.to - g.from + 1, peakTime: a.samples[g.from].Time}
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
			stats.peak, stats.peakTime = measure, a.samples[i].Time
		}
		if !a.groupActive(i, g, floors) {
			continue
		}
		stats.active++
		stats.union = stats.union.Union(box)
		stats.footprint += math.Hypot(float64(box.Dx()), float64(box.Dy()))
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

	if g.timescale() == slow {
		return a.gradualEvent(e, opt), true
	}
	return a.fastEvent(e, stats, opt), true
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

// wholeFrame reports an event large enough that calling it localised would
// mislead. Steps and blips are exempt: a one-off whole-frame change is a real,
// locatable moment even though it covers everything.
func wholeFrame(e Event) bool {
	return e.RegionArea > 0.6 && e.Kind != KindStep && e.Kind != KindBlip
}

func classify(e Event, stats groupStats) string {
	duty := float64(stats.active) / float64(stats.frames)
	// Brief means few transitions, not a short span: something that changes
	// and changes back is two transitions however far apart they are.
	brief := stats.active <= 2

	switch {
	case brief && truthy(e.Persists):
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
		if e.RegionArea > 0.6 {
			return fmt.Sprintf("Most of the frame (%s) differs from itself %.1fs earlier, throughout %s to %s. That is what continuous motion looks like over the slow window, not a single gradual change.",
				size, opt.DriftSeconds, clock(e.Start), clock(e.End))
		}
		return fmt.Sprintf("Gradual change from %s to %s in the %s (%s). Too slow to clear the threshold between adjacent frames; only visible over the %.1fs drift window.",
			clock(e.Start), clock(e.End), e.Position, size, opt.DriftSeconds)
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
		return fmt.Sprintf("Movement from %s to %s in the %s (%s); the active area travels %s across about %d px. The region is the whole path swept, not the size of the thing moving.",
			clock(e.Start), clock(e.End), e.Position, size, e.Direction, e.TravelPixels)
	default:
		if e.RegionArea > 0.6 {
			return fmt.Sprintf("Most of the frame (%s) is changing continuously from %s to %s. This is whole-frame motion rather than one localised event; its start and end are where activity crossed the noise floor, not real boundaries.",
				size, clock(e.Start), clock(e.End))
		}
		return fmt.Sprintf("Sustained activity from %s to %s in the %s (%s), averaging %.2f%% of the frame changing per step.",
			clock(e.Start), clock(e.End), e.Position, size, e.MeanChanged*100)
	}
}

func wholeFrameSummary(e Event) string {
	if e.Kind == KindFlash {
		return fmt.Sprintf("Whole-frame flash at %s lasting about %.0f ms; the picture then returns to what it was.",
			clock(e.Start), math.Max(1, (e.End-e.Start)*1000))
	}
	return fmt.Sprintf("Hard cut at %s: %.0f%% of the frame changes in a single transition and stays changed.",
		clock(e.Start), e.PeakChanged*100)
}

// groupSample reduces one transition to what this group of cells saw.
func (a *Analyzer) groupSample(i int, g group) (changed, drift float64, box image.Rectangle, centre image.Point) {
	t := g.timescale()
	var count, driftCount int32
	var weightX, weightY, weight float64
	for _, c := range g.cells {
		cell := a.samples[i].Cells[c]
		count += cell.Changed
		driftCount += cell.Drift
		b := t.box(cell)
		if b.Empty() {
			continue
		}
		box = box.Union(b)
		w := float64(t.count(cell))
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
	t := g.timescale()
	for _, c := range g.cells {
		if a.cellMeasure(t, i, c) > floors[c] {
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
