package motion

import (
	"fmt"
	"image"
	"math"
)

// describe builds an event from one group of cells over one stretch of samples.
func (a *Analyzer) describe(g group, floors []float64, opt TimelineOptions) (Event, bool) {
	e := Event{Start: a.samples[g.from].Time, End: a.samples[g.to].Time, Peak: a.samples[g.from].Time}

	union := image.Rectangle{}
	active := 0
	var sum float64
	var firstCentre, lastCentre image.Point
	var footprint float64
	haveFirst := false

	for i := g.from; i <= g.to; i++ {
		changed, drift, box, centre := a.groupSample(i, g)
		sum += changed
		e.PeakDrift = math.Max(e.PeakDrift, drift)
		measure := changed
		if g.slow {
			measure = drift
		}
		if measure > e.PeakChanged {
			e.PeakChanged, e.Peak = measure, a.samples[i].Time
		}
		if !a.groupActive(i, g, floors) {
			continue
		}
		active++
		union = union.Union(box)
		footprint += math.Hypot(float64(box.Dx()), float64(box.Dy()))
		if !haveFirst {
			firstCentre, haveFirst = centre, true
		}
		lastCentre = centre
	}
	if union.Empty() {
		return Event{}, false
	}
	e.MeanChanged = sum / float64(g.to-g.from+1)
	e.Region = a.sourceRect(union, opt)
	e.RegionArea = areaFraction(e.Region, opt)
	e.Position = positionOf(e.Region, opt)
	e.Persists = a.persists(union, e.Start, e.End)

	duration := e.End - e.Start
	if g.slow {
		e.Kind = KindGradual
		e.Start = math.Max(0, e.Start-opt.DriftSeconds)
		e.round()
		if e.RegionArea > 0.6 {
			e.Summary = fmt.Sprintf("Most of the frame (%s) differs from itself %.1fs earlier, throughout %s to %s. That is what continuous motion looks like over the slow window, not a single gradual change.",
				regionSize(e.Region), opt.DriftSeconds, clock(e.Start), clock(e.End))
			return e, true
		}
		e.Summary = fmt.Sprintf(
			"Gradual change from %s to %s in the %s (%s). Too slow to clear the threshold between adjacent frames; only visible over the %.1fs drift window.",
			clock(e.Start), clock(e.End), e.Position, regionSize(e.Region), opt.DriftSeconds)
		return e, true
	}

	e.Direction, e.TravelPixels = a.travel(firstCentre, lastCentre, footprint, active, opt)
	e.Kind = classify(e, active, g.to-g.from+1, duration, opt)
	if e.RegionArea > 0.6 && e.Kind != KindStep && e.Kind != KindBlip {
		// A "flicker across the whole frame" is not a finding, it is a
		// description of continuous motion. Say that instead.
		e.Kind, e.ChangesPerSecond, e.Direction, e.TravelPixels = KindBusy, 0, "", 0
		e.round()
		e.Summary = fmt.Sprintf("Most of the frame (%s) is changing continuously from %s to %s. This is whole-frame motion rather than one localised event; its start and end are where activity crossed the noise floor, not real boundaries.",
			regionSize(e.Region), clock(e.Start), clock(e.End))
		return e, true
	}
	if e.Kind == KindFlicker && duration > 0 {
		e.ChangesPerSecond = math.Round(float64(active)/duration*10) / 10
	}
	e.round()
	e.Summary = summarise(e, duration)
	return e, true
}

// groupSample reduces one transition to what this group of cells saw.
func (a *Analyzer) groupSample(i int, g group) (changed, drift float64, box image.Rectangle, centre image.Point) {
	var count, driftCount int32
	var weightX, weightY, weight float64
	for _, c := range g.cells {
		cell := a.samples[i].Cells[c]
		count += cell.Changed
		driftCount += cell.Drift
		b := cell.Box()
		if g.slow {
			b = cell.DriftBox()
		}
		if b.Empty() {
			continue
		}
		box = box.Union(b)
		w := float64(cell.Changed)
		if g.slow {
			w = float64(cell.Drift)
		}
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
	for _, c := range g.cells {
		measure := a.cellChanged(i, c)
		if g.slow {
			measure = a.cellDrift(i, c)
		}
		if measure > floors[c] {
			return true
		}
	}
	return false
}

func classify(e Event, active, frames int, _ float64, _ TimelineOptions) string {
	duty := float64(active) / float64(frames)
	// Brief means few transitions, not a short span: something that changes
	// and changes back is two transitions however far apart they are.
	brief := active <= 2

	switch {
	case brief && truthy(e.Persists):
		return KindStep
	case brief:
		return KindBlip
	case active >= 4 && duty < 0.75 && e.TravelPixels == 0:
		return KindFlicker
	case e.TravelPixels > 0:
		return KindMotion
	default:
		return KindBusy
	}
}

func summarise(e Event, duration float64) string {
	size := regionSize(e.Region)
	switch e.Kind {
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
		return fmt.Sprintf("Movement from %s to %s in the %s (%s); the active area travels %s across about %d px.",
			clock(e.Start), clock(e.End), e.Position, size, e.Direction, e.TravelPixels)
	default:
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

// travel names the direction only when the centre moves further than the
// typical per-frame footprint, so a stationary flicker is never called motion.
func (a *Analyzer) travel(first, last image.Point, footprint float64, active int, opt TimelineOptions) (string, int) {
	if active < 2 {
		return "", 0
	}
	sx, sy := a.scale(opt)
	dx := float64(last.X-first.X) * sx
	dy := float64(last.Y-first.Y) * sy
	distance := math.Hypot(dx, dy)
	if distance < (footprint/float64(active))*math.Max(sx, sy) {
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
	case horizontal != "" && vertical != "":
		return horizontal + " and " + vertical
	case horizontal != "":
		return horizontal
	default:
		return vertical
	}
}

// round trims float noise so the record stays readable and diffs cleanly.
func (e *Event) round() {
	e.Start, e.End, e.Peak = round2(e.Start), round2(e.End), round2(e.Peak)
	e.PeakChanged, e.MeanChanged = round4(e.PeakChanged), round4(e.MeanChanged)
	e.PeakDrift, e.RegionArea = round4(e.PeakDrift), round4(e.RegionArea)
}
