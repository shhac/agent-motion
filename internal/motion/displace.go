package motion

import (
	"image"
	"math"
	"sort"
)

// Displacement is an axis-aligned translation of content between two frames,
// in analysis pixels.
type Displacement struct {
	DX, DY int
	// Confidence is how much better the translated match is than assuming
	// nothing moved: 0 means no better at all, 1 means a perfect match against
	// a completely different starting point.
	Confidence float64
}

// Moved reports whether anything actually moved.
func (d Displacement) Moved() bool { return d.DX != 0 || d.DY != 0 }

// minShiftConfidence is how much better a translated match must be before the
// change is called a move rather than an appearance. Content that genuinely
// slid produces a near-perfect match at its offset; content that appeared
// produces no improvement at any offset.
const minShiftConfidence = 0.55

// minAxisGain is how much better, in luminance units, the best offset must be
// than not moving at all.
//
// The gate is on the absolute gain rather than on the starting difference,
// because the two are worlds apart. An axis that genuinely moved gains a lot
// even for a tiny shift — a 2px slide of a 122px block gains 0.65 — while an
// axis that did not move starts at almost zero, where the relative improvement
// is meaningless and codec noise alone invents a large phantom offset. Measured
// in TestAxisGainSeparatesMovementFromNoise.
const minAxisGain = 0.25

// A translation leaves almost nothing behind: after[i] is before[i-d], so what
// is left over should be small next to the variation in the signal itself.
// Measured on a real browser reflow — a 600x200 image with no dimensions
// loading late, pushing the page down exactly 200px — the true vertical offset
// leaves a residual of 0.05 of the profile's spread, while a spurious
// horizontal match on the same frames leaves 3.3 of it.
const maxResidualShare = 0.4

// minProfileSpread is how much variation a profile needs before an offset in it
// means anything. A blank page before first paint has a spread of exactly zero
// and can be "translated" onto anything, which is how content appearing came to
// be reported as content moving.
const minProfileSpread = 2.0

// Translation finds how far the content inside a region moved between two
// frames.
//
// A layout shift is the same content in a new place, which makes this a
// registration problem rather than a detection one — and the tool has no other
// way to tell "this moved" from "this appeared", which is the difference
// between a bug and a page working normally.
//
// It correlates one-dimensional brightness profiles, mean luminance per row and
// per column, rather than matching blocks. That costs O(size + search) instead
// of O(size x search), and it fits the domain: a page is rows of text and
// stacked blocks, and the shifts worth finding are axis aligned.
func Translation(before, after image.Image, region image.Rectangle, limit int) Displacement {
	region = region.Intersect(before.Bounds()).Intersect(after.Bounds())
	if region.Dx() < 4 || region.Dy() < 4 || limit < 1 {
		return Displacement{}
	}
	dy, confidenceY := bestOffset(
		rowProfile(before, region), rowProfile(after, region), min(limit, region.Dy()/2))
	dx, confidenceX := bestOffset(
		colProfile(before, region), colProfile(after, region), min(limit, region.Dx()/2))

	// Take the confidence from whichever axis actually moved. A purely vertical
	// shift leaves the column profile unchanged, and scoring that as "no
	// confidence" would reject the very case this exists for.
	confidence := 0.0
	if dy != 0 {
		confidence = math.Max(confidence, confidenceY)
	}
	if dx != 0 {
		confidence = math.Max(confidence, confidenceX)
	}
	if confidence < minShiftConfidence {
		return Displacement{}
	}
	return Displacement{DX: dx, DY: dy, Confidence: round2(confidence)}
}

// bestOffset finds d minimising the difference between after[i] and
// before[i-d], with d positive meaning the content moved forward along the axis.
func bestOffset(before, after []float64, limit int) (int, float64) {
	zero := profileDistance(before, after, 0)
	bestD, best := 0, zero
	for d := -limit; d <= limit; d++ {
		if d == 0 {
			continue
		}
		if distance := profileDistance(before, after, d); distance < best {
			bestD, best = d, distance
		}
	}
	spread := spreadOf(before)
	switch {
	case bestD == 0, zero-best < minAxisGain:
		return 0, 0
	case spread < minProfileSpread:
		return 0, 0 // nothing to register against; any offset would fit
	case best > maxResidualShare*spread:
		return 0, 0 // the offset does not actually explain the change
	}
	return bestD, 1 - best/zero
}

// profileDistance is the mean absolute difference between two profiles at an
// offset. Averaging rather than summing matters: a larger offset overlaps on
// fewer samples, and an unnormalised sum would make every large shift look like
// a better match than a small one.
func profileDistance(before, after []float64, d int) float64 {
	total, count := 0.0, 0
	for i := range after {
		j := i - d
		if j < 0 || j >= len(before) {
			continue
		}
		total += math.Abs(after[i] - before[j])
		count++
	}
	if count < len(after)/2 {
		return math.MaxFloat64 // too little overlap to mean anything
	}
	return total / float64(count)
}

func rowProfile(frame image.Image, r image.Rectangle) []float64 {
	out := make([]float64, r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		total := 0.0
		for x := r.Min.X; x < r.Max.X; x++ {
			total += luma(frame, x, y)
		}
		out[y-r.Min.Y] = total / float64(r.Dx())
	}
	return out
}

func colProfile(frame image.Image, r image.Rectangle) []float64 {
	out := make([]float64, r.Dx())
	for x := r.Min.X; x < r.Max.X; x++ {
		total := 0.0
		for y := r.Min.Y; y < r.Max.Y; y++ {
			total += luma(frame, x, y)
		}
		out[x-r.Min.X] = total / float64(r.Dy())
	}
	return out
}

func luma(frame image.Image, x, y int) float64 {
	r, g, b, _ := frame.At(x, y).RGBA()
	return (float64(r>>8) + float64(g>>8) + float64(b>>8)) / 3
}

// shiftBudget caps how many events are worth fetching frames for. Deciding a
// shift costs two decoded frames, and on footage where everything moves there
// can be dozens of candidates that are not worth the wait.
const shiftBudget = 12

// FrameAt fetches the source frame shown at a timestamp.
type FrameAt func(at float64) (image.Image, error)

// ResolveShifts decides which brief changes were content moving rather than
// content appearing, and rewrites them.
//
// Nothing in the statistics can tell those apart — both are one transition in
// one place — yet on a page the difference is the entire question. A banner
// that appears is the page working; a banner that shoves the article down is
// the bug. Answering it needs the actual pixels either side of the transition,
// so frames are fetched on demand rather than kept from the analysis pass: a
// shift is often one of the *smallest* changes in a recording, and anything
// retained by size would have thrown it away.
//
// Events that begin at the same instant are also tried together. A block
// sliding sideways changes only its two vertical edges, which can be hundreds
// of pixels apart and are correctly two events — but separately they are two
// unreadable hairlines, and together they are one rectangle that moved.
func ResolveShifts(events []Event, opt TimelineOptions, fps float64, frame FrameAt) []Event {
	if frame == nil || fps <= 0 {
		return events
	}
	// Event times are rounded to hundredths for readability, and seeking snaps
	// to the frame at or after the time asked for. Asking for exactly one frame
	// either side of a rounded time therefore lands both requests on the same
	// side of the transition — which reads as "nothing changed" and is the
	// worst possible answer. Half-frame margins put each request unambiguously
	// on its own side.
	const beforeMargin, afterMargin = 1.5, 0.25

	cache := map[float64]image.Image{}
	at := func(t float64) image.Image {
		if img, seen := cache[t]; seen {
			return img
		}
		img, err := frame(t)
		if err != nil {
			img = nil
		}
		cache[t] = img
		return img
	}

	resolveWholeFrame(events, opt, fps, at, beforeMargin, afterMargin)

	order := brieflyChangedOrder(events)
	resolved := make([]bool, len(events))
	out := make([]Event, len(events))
	copy(out, events)
	spent := 0

	for _, i := range order {
		if spent >= shiftBudget {
			break
		}
		if resolved[i] {
			continue
		}
		before, after := at(events[i].Peak-beforeMargin/fps), at(events[i].Peak+afterMargin/fps)
		if before == nil || after == nil {
			continue
		}
		spent++

		// Try this event with each partner that began at the same instant,
		// widest first, then alone.
		partner := -1
		region := rect(events[i].Region)
		for _, j := range order {
			if j == i || resolved[j] || events[j].Peak != events[i].Peak {
				continue
			}
			union := region.Union(rect(events[j].Region))
			if area(union) < 2*max(area(region), area(rect(events[j].Region))) {
				continue
			}
			if moved := measureShift(before, after, union, opt); moved.Moved() {
				out[i] = asShift(events[i], union, moved, opt)
				resolved[i], resolved[j] = true, true
				partner = j
				break
			}
		}
		if partner >= 0 {
			continue
		}
		if moved := measureShift(before, after, region, opt); moved.Moved() {
			out[i] = asShift(events[i], region, moved, opt)
			resolved[i] = true
		}
	}

	kept := out[:0]
	for i := range out {
		// A joined pair leaves its partner behind: it is now described by the
		// event it merged into.
		if resolved[i] && out[i].Kind != KindShift {
			continue
		}
		kept = append(kept, out[i])
	}
	return kept
}

// resolveWholeFrame asks of every cut and flash whether the picture actually
// changed or merely changed brightness. Both look the same in the statistics,
// and it is the most dramatic event in a recording — the one most likely to be
// read as the headline and most expensive to check by hand.
func resolveWholeFrame(events []Event, opt TimelineOptions, fps float64, at func(float64) image.Image, beforeMargin, afterMargin float64) {
	whole := image.Rect(0, 0, opt.SourceWidth, opt.SourceHeight)
	for i := range events {
		// Only cuts. A flash returns to what it was, so "did the content change
		// or only its brightness" has no meaning for one, and its single odd
		// frame is skipped by the margins anyway.
		if events[i].Kind != KindCut {
			continue
		}
		before, after := at(events[i].Peak-beforeMargin/fps), at(events[i].Peak+afterMargin/fps)
		if before == nil || after == nil {
			continue
		}
		residual, scale, uniform := uniformShade(before, after, whole)
		events[i].ShadeResidual, events[i].ShadeScale, events[i].Uniform = residual, scale, uniform
		events[i].Summary = wholeFrameSummary(events[i])
	}
}

func measureShift(before, after image.Image, region image.Rectangle, opt TimelineOptions) Displacement {
	span := max(opt.SourceWidth, opt.SourceHeight)
	limit := min(max(region.Dx(), region.Dy())/2, span/3)
	return Translation(before, after, region, limit)
}

func asShift(e Event, region image.Rectangle, moved Displacement, opt TimelineOptions) Event {
	e.Region = [4]int{region.Min.X, region.Min.Y, region.Max.X, region.Max.Y}
	e.RegionArea = areaFraction(e.Region, opt)
	e.Position = positionOf(e.Region, opt)
	e.Kind = KindShift
	e.MovedBy = []int{moved.DX, moved.DY}
	e.ShiftScore = shiftScore(e.RegionArea, moved.DX, moved.DY, opt)
	e.round()
	e.Summary = summarise(e, opt)
	return e
}

// brieflyChangedOrder lists the candidate events, most prominent first, so a
// capped budget is spent on the changes that matter most.
//
// Whole-frame changes are deliberately not candidates. Tried against eight real
// page loads, a browser re-layout never registered as a translation — content
// re-wraps and resizes rather than sliding — so including them found nothing
// and spent the frame budget ahead of the smaller events that do translate.
func brieflyChangedOrder(events []Event) []int {
	var order []int
	for i, e := range events {
		if e.Kind == KindStep || e.Kind == KindBlip {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool {
		return prominence(events[order[a]]) > prominence(events[order[b]])
	})
	return order
}

// shiftScore is CLS-shaped: how much of the frame was affected, times how far
// it went as a share of the frame. It deliberately is not called CLS — the real
// measure comes from the DOM, covers a session window, and knows which elements
// are unstable. This is what can be had from pixels alone.
func shiftScore(area float64, dx, dy int, opt TimelineOptions) float64 {
	span := max(opt.SourceWidth, opt.SourceHeight)
	if span == 0 {
		return 0
	}
	return round4(area * float64(max(abs(dx), abs(dy))) / float64(span))
}

func rect(r [4]int) image.Rectangle { return image.Rect(r[0], r[1], r[2], r[3]) }

func area(r image.Rectangle) int { return r.Dx() * r.Dy() }

// spreadOf is the standard deviation of a profile: how much signal there is to
// register against.
func spreadOf(p []float64) float64 {
	if len(p) == 0 {
		return 0
	}
	var sum, sumSq float64
	for _, v := range p {
		sum += v
		sumSq += v * v
	}
	n := float64(len(p))
	return math.Sqrt(math.Max(0, sumSq/n-(sum/n)*(sum/n)))
}
