package motion

import (
	"fmt"
	"image"
	"math"
	"sort"
)

// Event kinds. They describe the shape of a change, never its meaning.
const (
	KindCut     = "cut"     // one transition replaces most of the frame and it stays replaced
	KindFlash   = "flash"   // most of the frame changes for one or two frames, then returns
	KindStep    = "step"    // brief localised change that is still there afterwards
	KindBlip    = "blip"    // brief localised change that reverts
	KindFlicker = "flicker" // the same area toggles repeatedly
	KindMotion  = "motion"  // activity whose centre travels across the frame
	KindGradual = "gradual" // too slow to see frame to frame, clear over the drift window
	KindBusy    = "busy"    // sustained activity with no clearer shape
)

// Event is one described occurrence in source-video coordinates and seconds.
type Event struct {
	Kind        string  `json:"kind"`
	Start       float64 `json:"start_seconds"`
	End         float64 `json:"end_seconds"`
	Peak        float64 `json:"peak_seconds"`
	PeakChanged float64 `json:"peak_changed_fraction"`
	MeanChanged float64 `json:"mean_changed_fraction"`
	// PeakDrift is the strongest difference across the slow window. It is the
	// only non-zero measure for changes too slow to see frame to frame.
	PeakDrift  float64 `json:"peak_drift_fraction,omitempty"`
	Region     [4]int  `json:"region_xyxy"`
	RegionArea float64 `json:"region_area_fraction"`
	Position   string  `json:"position"`
	// ChangesPerSecond is set for repeating events.
	ChangesPerSecond float64 `json:"changes_per_second,omitempty"`
	// Direction and TravelPixels are set when the active centre moves.
	Direction    string `json:"direction,omitempty"`
	TravelPixels int    `json:"travel_pixels,omitempty"`
	// Persists reports whether the region still looks different afterwards.
	// It is nil when there was no checkpoint to compare against.
	Persists *bool  `json:"persists,omitempty"`
	Summary  string `json:"summary"`
}

// Timeline is the described result of one analysis pass.
type Timeline struct {
	Events     []Event    `json:"events"`
	NoiseFloor float64    `json:"noise_floor_fraction"`
	Truncated  int        `json:"events_omitted,omitempty"`
	Fit        Assessment `json:"suitability"`
}

// Suitability verdicts.
const (
	FitGood     = "suitable"
	FitMarginal = "marginal"
	FitPoor     = "unsuitable"
)

// Assessment says how much this recording resembles the fixed-viewport footage
// the tool works on. Without it, continuously moving footage produces a long
// list of confident-sounding events that mean nothing, and a caller has no way
// to tell that from a real finding.
type Assessment struct {
	Verdict string `json:"verdict"`
	// TypicalChanged is the median share of the frame changing per transition.
	// It has to be an absolute measure: the per-cell noise floors adapt to the
	// recording, so a relative one normalises away the very thing being asked.
	TypicalChanged float64 `json:"typical_changed_fraction"`
	Reason         string  `json:"reason"`
	Advice         string  `json:"advice,omitempty"`
}

// assess measures how much of the frame is in motion at a typical moment.
func (a *Analyzer) assess() Assessment {
	if len(a.samples) == 0 {
		return Assessment{Verdict: FitGood, Reason: "Nothing to measure."}
	}
	changed := make([]float64, len(a.samples))
	for i, s := range a.samples {
		changed[i] = s.Changed
	}
	typical := round4(median(changed))

	switch {
	case typical > 0.25:
		return Assessment{
			Verdict: FitPoor, TypicalChanged: typical,
			Reason: fmt.Sprintf("In a typical frame %.0f%% of the picture changes. That is what a moving camera, a scrolling page, or full-motion video looks like, not a fixed viewport.", typical*100),
			Advice: "Treat the events below as unreliable: where most of the frame moves at once, the boundaries between events are arbitrary and small findings are fragments of one moving scene. Use 'sheet' to look at the content instead, and 'frames' for specific moments.",
		}
	case typical > 0.06:
		return Assessment{
			Verdict: FitMarginal, TypicalChanged: typical,
			Reason: fmt.Sprintf("In a typical frame %.0f%% of the picture changes, which is a lot for a fixed viewport.", typical*100),
			Advice: "Some events may be fragments of one continuously moving thing rather than separate findings. Check a 'sheet' before relying on the event list.",
		}
	default:
		return Assessment{
			Verdict: FitGood, TypicalChanged: typical,
			Reason: fmt.Sprintf("Most of the frame holds still; %.2f%% of it changes in a typical frame.", typical*100),
		}
	}
}

// TimelineOptions tunes segmentation. Zero values take documented defaults.
type TimelineOptions struct {
	FPS                       float64
	SourceWidth, SourceHeight int
	DriftSeconds              float64
	CutFraction               float64
	MergeGap                  float64
	MinFloor                  float64
	MaxEvents                 int
}

func (o TimelineOptions) withDefaults() TimelineOptions {
	if o.CutFraction <= 0 {
		o.CutFraction = 0.5
	}
	if o.MergeGap <= 0 {
		o.MergeGap = 0.25
	}
	if o.MinFloor <= 0 {
		o.MinFloor = 0.0015
	}
	if o.MaxEvents <= 0 {
		o.MaxEvents = 40
	}
	return o
}

type span struct{ from, to int } // inclusive sample indices

// cellSpan is one cell being active over one stretch of time.
type cellSpan struct {
	cell int
	span
	slow bool
}

// group is a set of touching cells active over the same stretch, which is what
// becomes a single event.
type group struct {
	cells []int
	span
	slow bool
}

// Timeline segments the series in space and time, then describes each segment.
//
// Segmentation is per grid cell rather than per frame because real recordings
// almost always have something animating continuously — a spinner, a cursor, a
// clock. A purely temporal pass merges that with everything else and reports
// one enormous event covering the whole video.
func (a *Analyzer) Timeline(opt TimelineOptions) Timeline {
	opt = opt.withDefaults()
	if len(a.samples) == 0 {
		return Timeline{Events: []Event{}}
	}
	consumed := make([]bool, len(a.samples))
	events := a.wholeFrameEvents(opt, consumed)

	floors := a.cellFloors(opt.MinFloor)
	gap := a.gapSamples(opt.MergeGap, opt.FPS)
	spans := a.fastSpans(floors, consumed, gap)
	spans = append(spans, a.slowSpans(floors, consumed, gap, opt)...)

	for _, g := range groupSpans(spans, a.grid, gap) {
		if e, ok := a.describe(g, floors, opt); ok {
			events = append(events, e)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Start < events[j].Start })

	t := Timeline{NoiseFloor: round4(median(floors)), Events: events, Fit: a.assess()}
	if len(events) > opt.MaxEvents {
		t.Events = trimToBudget(events, opt.MaxEvents)
		t.Truncated = len(events) - len(t.Events)
	}
	return t
}

func (a *Analyzer) gapSamples(gap, fps float64) int {
	if fps <= 0 {
		return 1
	}
	return max(1, int(math.Round(gap*fps)))
}

// cellFloors estimates per-cell noise. A small event fills a much larger share
// of one cell than of the whole frame, which is why sensitivity improves when
// the floor is local.
func (a *Analyzer) cellFloors(minimum float64) []float64 {
	floors := make([]float64, len(a.grid.Pixels))
	series := make([]float64, len(a.samples))
	for c := range floors {
		for i, s := range a.samples {
			series[i] = float64(s.Cells[c].Changed) / float64(a.grid.Pixels[c])
		}
		// Two changed pixels is the smallest thing worth calling an event; one
		// is indistinguishable from a single noisy pixel.
		floors[c] = math.Max(robustFloor(series, minimum), 2/float64(a.grid.Pixels[c]))
	}
	return floors
}

func (a *Analyzer) fastSpans(floors []float64, consumed []bool, gap int) []cellSpan {
	var out []cellSpan
	for c := range floors {
		for _, sp := range runs(len(a.samples), func(i int) bool {
			return !consumed[i] && a.cellChanged(i, c) > floors[c]
		}, gap) {
			out = append(out, cellSpan{cell: c, span: sp})
		}
	}
	return out
}

// slowSpans finds cells drifting without any fast activity of their own. The
// mask is per cell, so a constantly animating corner no longer hides a slow
// change happening elsewhere.
func (a *Analyzer) slowSpans(floors []float64, consumed []bool, gap int, opt TimelineOptions) []cellSpan {
	if opt.DriftSeconds <= 0 || opt.FPS <= 0 {
		return nil
	}
	lag := int(math.Round(opt.DriftSeconds * opt.FPS))
	var out []cellSpan
	for c := range floors {
		fast := make([]bool, len(a.samples))
		for i := range a.samples {
			if !consumed[i] && a.cellChanged(i, c) <= floors[c] {
				continue
			}
			// Drift stays elevated for one window after any real change.
			for j := i; j <= min(len(a.samples)-1, i+lag); j++ {
				fast[j] = true
			}
		}
		for _, sp := range runs(len(a.samples), func(i int) bool {
			return !fast[i] && a.cellDrift(i, c) > floors[c]
		}, lag) {
			if sp.to-sp.from+1 >= max(2, lag) {
				out = append(out, cellSpan{cell: c, span: sp, slow: true})
			}
		}
	}
	return out
}

func (a *Analyzer) cellChanged(i, c int) float64 {
	return float64(a.samples[i].Cells[c].Changed) / float64(a.grid.Pixels[c])
}

func (a *Analyzer) cellDrift(i, c int) float64 {
	return float64(a.samples[i].Cells[c].Drift) / float64(a.grid.Pixels[c])
}

// groupSpans merges touching cells whose active stretches overlap, so one thing
// happening across several cells is one event and two things happening at once
// in different places stay two.
func groupSpans(spans []cellSpan, grid Grid, gap int) []group {
	parent := make([]int, len(spans))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	for i := range spans {
		for j := i + 1; j < len(spans); j++ {
			if spans[i].slow != spans[j].slow {
				continue
			}
			if !grid.Adjacent(spans[i].cell, spans[j].cell) {
				continue
			}
			if !overlaps(spans[i].span, spans[j].span, gap) {
				continue
			}
			parent[find(i)] = find(j)
		}
	}
	byRoot := map[int]*group{}
	var order []int
	for i, s := range spans {
		root := find(i)
		g, ok := byRoot[root]
		if !ok {
			g = &group{span: span{from: s.from, to: s.to}, slow: s.slow}
			byRoot[root] = g
			order = append(order, root)
		}
		g.cells = append(g.cells, s.cell)
		g.from, g.to = min(g.from, s.from), max(g.to, s.to)
	}
	out := make([]group, 0, len(order))
	for _, root := range order {
		g := byRoot[root]
		sort.Ints(g.cells)
		g.cells = dedupe(g.cells)
		out = append(out, *g)
	}
	return out
}

func overlaps(a, b span, gap int) bool {
	return a.from-gap <= b.to && b.from-gap <= a.to
}

// wholeFrameEvents pulls out cuts and flashes before anything else, so one
// enormous transition does not swallow the events around it.
func (a *Analyzer) wholeFrameEvents(opt TimelineOptions, consumed []bool) []Event {
	s := a.samples
	var events []Event
	for _, sp := range runs(len(s), func(i int) bool { return s[i].Changed >= opt.CutFraction }, 1) {
		if sp.to-sp.from+1 > 2 {
			continue // a long whole-frame run is sustained activity, not a boundary
		}
		for i := sp.from; i <= sp.to; i++ {
			consumed[i] = true
		}
		whole := image.Rect(0, 0, a.width, a.height)
		e := Event{
			Kind: KindCut, Start: s[sp.from].Time, End: s[sp.to].Time, Peak: s[sp.from].Time,
			Region: a.sourceRect(whole, opt), RegionArea: 1, Position: "whole frame",
			Persists: a.persists(whole, s[sp.from].Time, s[sp.to].Time),
		}
		for i := sp.from; i <= sp.to; i++ {
			e.PeakChanged = math.Max(e.PeakChanged, s[i].Changed)
			e.MeanChanged += s[i].Changed / float64(sp.to-sp.from+1)
		}
		if sp.to > sp.from || falsey(e.Persists) {
			e.Kind = KindFlash
		}
		e.round()
		e.Summary = wholeFrameSummary(e)
		events = append(events, e)
	}
	return events
}

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

// persists compares the nearest retained frames either side of a stretch to say
// whether the region ended up looking different.
func (a *Analyzer) persists(region image.Rectangle, start, end float64) *bool {
	before, after := a.checkpointBefore(start), a.checkpointAfter(end)
	if before == nil || after == nil || region.Empty() {
		return nil
	}
	r := a.checkpointRect(region)
	if r.Empty() {
		return nil
	}
	var total float64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := (y*a.cpWidth + x) * 3
			total += (absDiff(before.pix[i], after.pix[i]) +
				absDiff(before.pix[i+1], after.pix[i+1]) +
				absDiff(before.pix[i+2], after.pix[i+2])) / 3
		}
	}
	changed := total/float64(r.Dx()*r.Dy()) > math.Max(4, a.opt.Threshold/2)
	return &changed
}

func (a *Analyzer) checkpointBefore(t float64) *checkpoint {
	for i := len(a.checkpoints) - 1; i >= 0; i-- {
		if a.checkpoints[i].time < t {
			return &a.checkpoints[i]
		}
	}
	return nil
}

func (a *Analyzer) checkpointAfter(t float64) *checkpoint {
	for i := range a.checkpoints {
		if a.checkpoints[i].time > t {
			return &a.checkpoints[i]
		}
	}
	return nil
}

func (a *Analyzer) checkpointRect(r image.Rectangle) image.Rectangle {
	if a.cpWidth == 0 || a.cpHeight == 0 {
		return image.Rectangle{}
	}
	sx := float64(a.cpWidth) / float64(a.width)
	sy := float64(a.cpHeight) / float64(a.height)
	out := image.Rect(
		int(math.Floor(float64(r.Min.X)*sx)), int(math.Floor(float64(r.Min.Y)*sy)),
		int(math.Ceil(float64(r.Max.X)*sx)), int(math.Ceil(float64(r.Max.Y)*sy)),
	).Intersect(image.Rect(0, 0, a.cpWidth, a.cpHeight))
	if out.Dx() == 0 || out.Dy() == 0 {
		return image.Rectangle{}
	}
	return out
}

func (a *Analyzer) scale(opt TimelineOptions) (float64, float64) {
	sx, sy := 1.0, 1.0
	if opt.SourceWidth > 0 && a.width > 0 {
		sx = float64(opt.SourceWidth) / float64(a.width)
	}
	if opt.SourceHeight > 0 && a.height > 0 {
		sy = float64(opt.SourceHeight) / float64(a.height)
	}
	return sx, sy
}

func (a *Analyzer) sourceRect(r image.Rectangle, opt TimelineOptions) [4]int {
	sx, sy := a.scale(opt)
	return [4]int{
		int(math.Floor(float64(r.Min.X) * sx)), int(math.Floor(float64(r.Min.Y) * sy)),
		int(math.Ceil(float64(r.Max.X) * sx)), int(math.Ceil(float64(r.Max.Y) * sy)),
	}
}

func areaFraction(r [4]int, opt TimelineOptions) float64 {
	if opt.SourceWidth <= 0 || opt.SourceHeight <= 0 {
		return 0
	}
	return float64((r[2]-r[0])*(r[3]-r[1])) / float64(opt.SourceWidth*opt.SourceHeight)
}

// positionOf names the third of the frame the region sits in, which is easier
// to act on than four numbers.
func positionOf(r [4]int, opt TimelineOptions) string {
	if opt.SourceWidth <= 0 || opt.SourceHeight <= 0 {
		return "frame"
	}
	w, h := float64(opt.SourceWidth), float64(opt.SourceHeight)
	if float64(r[2]-r[0]) > 0.7*w && float64(r[3]-r[1]) > 0.7*h {
		return "whole frame"
	}
	cx := (float64(r[0]) + float64(r[2])) / 2 / w
	cy := (float64(r[1]) + float64(r[3])) / 2 / h
	return third(cy, "top", "middle", "bottom") + " " + third(cx, "left", "centre", "right")
}

func third(v float64, low, mid, high string) string {
	switch {
	case v < 1.0/3:
		return low
	case v < 2.0/3:
		return mid
	default:
		return high
	}
}

func regionSize(r [4]int) string {
	return fmt.Sprintf("%dx%d px at %d,%d", r[2]-r[0], r[3]-r[1], r[0], r[1])
}

func clock(t float64) string { return fmt.Sprintf("%.2fs", t) }

// round trims float noise so the record stays readable and diffs cleanly.
func (e *Event) round() {
	e.Start, e.End, e.Peak = round2(e.Start), round2(e.End), round2(e.Peak)
	e.PeakChanged, e.MeanChanged = round4(e.PeakChanged), round4(e.MeanChanged)
	e.PeakDrift, e.RegionArea = round4(e.PeakDrift), round4(e.RegionArea)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

// runs groups indices satisfying match into spans, joining spans separated by
// at most gap non-matching samples.
func runs(n int, match func(int) bool, gap int) []span {
	var out []span
	current := span{-1, -1}
	miss := 0
	for i := 0; i < n; i++ {
		if match(i) {
			if current.from < 0 {
				current.from = i
			}
			current.to = i
			miss = 0
			continue
		}
		if current.from < 0 {
			continue
		}
		miss++
		if miss > gap {
			out = append(out, current)
			current = span{-1, -1}
			miss = 0
		}
	}
	if current.from >= 0 {
		out = append(out, current)
	}
	return out
}

// robustFloor estimates noise as the median plus six median absolute
// deviations, which adapts to the recording instead of assuming a codec.
func robustFloor(values []float64, minimum float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	m := quantile(sorted, 0.5)
	deviations := make([]float64, len(sorted))
	for i, v := range sorted {
		deviations[i] = math.Abs(v - m)
	}
	sort.Float64s(deviations)
	return math.Min(0.25, math.Max(minimum, m+6*quantile(deviations, 0.5)))
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(math.Round(q*float64(len(sorted)-1)))]
}

func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return quantile(sorted, 0.5)
}

func dedupe(in []int) []int {
	out := in[:0]
	for i, v := range in {
		if i == 0 || v != in[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// trimToBudget keeps the most prominent events but preserves time order.
func trimToBudget(events []Event, budget int) []Event {
	ranked := append([]Event(nil), events...)
	sort.SliceStable(ranked, func(i, j int) bool {
		return math.Max(ranked[i].PeakChanged, ranked[i].PeakDrift) >
			math.Max(ranked[j].PeakChanged, ranked[j].PeakDrift)
	})
	keep := make(map[float64]bool, budget)
	for _, e := range ranked[:budget] {
		keep[e.Start] = true
	}
	out := make([]Event, 0, budget)
	for _, e := range events {
		if keep[e.Start] {
			out = append(out, e)
		}
	}
	return out
}

func truthy(b *bool) bool { return b != nil && *b }
func falsey(b *bool) bool { return b != nil && !*b }
