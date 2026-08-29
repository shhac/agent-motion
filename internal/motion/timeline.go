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
	Events     []Event `json:"events"`
	NoiseFloor float64 `json:"noise_floor_fraction"`
	Truncated  int     `json:"events_omitted,omitempty"`
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
		o.MinFloor = 0.0004
	}
	if o.MaxEvents <= 0 {
		o.MaxEvents = 40
	}
	return o
}

type span struct{ from, to int } // inclusive sample indices

// round trims float noise so the record stays readable and diffs cleanly.
func (e *Event) round() {
	e.Start = round2(e.Start)
	e.End = round2(e.End)
	e.Peak = round2(e.Peak)
	e.PeakChanged = round4(e.PeakChanged)
	e.MeanChanged = round4(e.MeanChanged)
	e.PeakDrift = round4(e.PeakDrift)
	e.RegionArea = round4(e.RegionArea)
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

// Timeline segments the per-transition series and describes each segment.
func (a *Analyzer) Timeline(opt TimelineOptions) Timeline {
	opt = opt.withDefaults()
	s := a.samples
	if len(s) == 0 {
		return Timeline{Events: []Event{}}
	}
	floor := noiseFloor(s, opt.MinFloor)
	consumed := make([]bool, len(s))

	events := a.wholeFrameEvents(opt, consumed)
	for _, sp := range runs(len(s), func(i int) bool {
		return !consumed[i] && s[i].Changed > floor
	}, a.gapSamples(opt.MergeGap, opt.FPS)) {
		events = append(events, a.describe(sp, floor, opt))
	}
	events = append(events, a.gradualEvents(opt, floor, consumed)...)

	sort.Slice(events, func(i, j int) bool { return events[i].Start < events[j].Start })
	t := Timeline{NoiseFloor: floor, Events: events}
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

// wholeFrameEvents pulls out cuts and flashes before ordinary segmentation, so
// one enormous transition does not swallow the events around it.
func (a *Analyzer) wholeFrameEvents(opt TimelineOptions, consumed []bool) []Event {
	s := a.samples
	var events []Event
	for _, sp := range runs(len(s), func(i int) bool { return s[i].Changed >= opt.CutFraction }, 1) {
		length := sp.to - sp.from + 1
		if length > 2 {
			continue // a long whole-frame run is sustained activity, not a boundary
		}
		for i := sp.from; i <= sp.to; i++ {
			consumed[i] = true
		}
		e := a.describe(sp, 0, opt)
		e.Kind = KindCut
		if length == 2 || falsey(e.Persists) {
			e.Kind = KindFlash
		}
		e.round()
		e.Summary = wholeFrameSummary(e)
		events = append(events, e)
	}
	return events
}

// gradualEvents finds change that is invisible frame to frame but obvious over
// the drift window, excluding time already explained by a faster event.
func (a *Analyzer) gradualEvents(opt TimelineOptions, floor float64, consumed []bool) []Event {
	if opt.DriftSeconds <= 0 || opt.FPS <= 0 {
		return nil
	}
	s := a.samples
	lag := int(math.Round(opt.DriftSeconds * opt.FPS))
	fast := make([]bool, len(s))
	for i := range s {
		if consumed[i] || s[i].Changed > floor {
			for j := i; j <= min(len(s)-1, i+lag); j++ {
				fast[j] = true // drift stays elevated for one window after real motion
			}
		}
	}
	var events []Event
	minLength := max(2, lag)
	for _, sp := range runs(len(s), func(i int) bool {
		return !fast[i] && s[i].Drift > floor
	}, lag) {
		if sp.to-sp.from+1 < minLength {
			continue
		}
		e := a.describe(sp, floor, opt)
		e.Kind = KindGradual
		e.Start = math.Max(0, e.Start-opt.DriftSeconds)
		e.ChangesPerSecond = 0
		e.Region = a.sourceRect(unionDriftBBox(s[sp.from:sp.to+1]), opt)
		e.RegionArea = areaFraction(e.Region, opt)
		e.Position = positionOf(e.Region, opt)
		e.Direction, e.TravelPixels = "", 0
		e.round()
		e.Summary = fmt.Sprintf(
			"Gradual change from %s to %s in the %s (%s). Too slow to clear the threshold between adjacent frames; only visible over the %.1fs drift window.",
			clock(e.Start), clock(e.End), e.Position, regionSize(e.Region), opt.DriftSeconds)
		events = append(events, e)
	}
	return events
}

// describe builds an event from one contiguous span of samples.
func (a *Analyzer) describe(sp span, floor float64, opt TimelineOptions) Event {
	s := a.samples[sp.from : sp.to+1]
	e := Event{
		Start: s[0].Time,
		End:   s[len(s)-1].Time,
		Peak:  s[0].Time,
	}
	var sum float64
	active := 0
	union := image.Rectangle{}
	for _, x := range s {
		sum += x.Changed
		e.PeakDrift = math.Max(e.PeakDrift, x.Drift)
		if x.Changed > e.PeakChanged {
			e.PeakChanged, e.Peak = x.Changed, x.Time
		}
		if x.Changed > floor {
			active++
			union = union.Union(x.BBox)
		}
	}
	e.MeanChanged = sum / float64(len(s))
	if union.Empty() {
		union = unionBBox(s)
	}
	e.Region = a.sourceRect(union, opt)
	e.RegionArea = areaFraction(e.Region, opt)
	e.Position = positionOf(e.Region, opt)
	e.Persists = a.persists(union, e.Start, e.End)

	duration := e.End - e.Start
	direction, travel := a.travel(s, floor, opt)
	e.Direction, e.TravelPixels = direction, travel
	e.Kind = classify(e, s, active, duration, opt)
	if e.Kind == KindFlicker && duration > 0 {
		e.ChangesPerSecond = math.Round(float64(active)/duration*10) / 10
	}
	e.round()
	e.Summary = summarise(e, duration)
	return e
}

func classify(e Event, s []Sample, active int, duration float64, opt TimelineOptions) string {
	duty := float64(active) / float64(len(s))
	brief := opt.FPS > 0 && duration <= 2.5/opt.FPS

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
		return fmt.Sprintf("Brief change at %s in the %s (%s) that reverts immediately.",
			clock(e.Start), e.Position, size)
	case KindFlicker:
		return fmt.Sprintf("Repeated toggling from %s to %s in the %s (%s), about %.1f changes per second over %.2fs.",
			clock(e.Start), clock(e.End), e.Position, size, e.ChangesPerSecond, duration)
	case KindMotion:
		return fmt.Sprintf("Movement from %s to %s in the %s (%s); the active area travels %s across about %d px.",
			clock(e.Start), clock(e.End), e.Position, size, e.Direction, e.TravelPixels)
	default:
		return fmt.Sprintf("Sustained activity from %s to %s in the %s (%s), averaging %.1f%% of pixels changing per frame.",
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

// travel measures how far the activity centre moves across the span, in source
// pixels, and names the direction when the movement is larger than the typical
// per-frame footprint.
func (a *Analyzer) travel(s []Sample, floor float64, opt TimelineOptions) (string, int) {
	var first, last *Sample
	var footprint float64
	count := 0
	for i := range s {
		if s[i].Changed <= floor {
			continue
		}
		if first == nil {
			first = &s[i]
		}
		last = &s[i]
		footprint += math.Hypot(float64(s[i].BBox.Dx()), float64(s[i].BBox.Dy()))
		count++
	}
	if first == nil || last == nil || count < 2 {
		return "", 0
	}
	sx, sy := a.scale(opt)
	dx := (last.CX - first.CX) * sx
	dy := (last.CY - first.CY) * sy
	distance := math.Hypot(dx, dy)
	if distance < (footprint/float64(count))*math.Max(sx, sy) {
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

// persists compares the nearest retained frames either side of a span to say
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
	mean := total / float64(r.Dx()*r.Dy())
	changed := mean > math.Max(4, a.opt.Threshold/2)
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

// noiseFloor is a robust estimate of per-frame codec and capture noise.
func noiseFloor(s []Sample, minimum float64) float64 {
	values := make([]float64, len(s))
	for i, x := range s {
		values[i] = x.Changed
	}
	sort.Float64s(values)
	median := quantile(values, 0.5)
	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - median)
	}
	sort.Float64s(deviations)
	floor := median + 6*quantile(deviations, 0.5)
	return math.Min(0.05, math.Max(minimum, floor))
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Round(q * float64(len(sorted)-1)))
	return sorted[i]
}

func unionBBox(s []Sample) image.Rectangle {
	out := image.Rectangle{}
	for _, x := range s {
		out = out.Union(x.BBox)
	}
	return out
}

func unionDriftBBox(s []Sample) image.Rectangle {
	out := image.Rectangle{}
	for _, x := range s {
		out = out.Union(x.DriftBBox)
	}
	return out
}

// trimToBudget keeps the most prominent events but preserves time order.
func trimToBudget(events []Event, budget int) []Event {
	ranked := append([]Event(nil), events...)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].PeakChanged > ranked[j].PeakChanged
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
