package motion

// The pass that needs real pixels. Everything here reads decoded frames back
// after the analysis has finished, to answer two questions the statistics
// cannot: did that content move, or is it new — and did the whole frame change
// picture, or only brightness. Both are worth a second decode because both are
// the difference between a bug and a page working normally, and both are
// expensive to check by hand.
//
// The measurement itself lives in displace.go, which knows nothing about
// events; this file is what turns it into a timeline.

import (
	"image"
	"sort"
)

// settleMargin is how far clear of a whole-frame fade its comparison is taken,
// in frames. The event's bounds are where activity crossed the noise floor, and
// an animated transition is still moving on either side of them — sampling at
// the edge catches a modal half closed, which fits no brightness map at all.
const settleMargin = 5.0

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

	// Copy before anything is written, not after. resolveWholeFrame fills in
	// the shade fields in place, so running it on the argument left the
	// caller's slice half-updated by a function whose signature promises a new
	// one. There is one caller and it reassigns, so nothing was wrong — but a
	// second caller would have inherited an aliasing bug for free.
	out := make([]Event, len(events))
	copy(out, events)

	resolveWholeFrame(out, opt, fps, at)

	order := brieflyChangedOrder(out, opt)
	resolved := make([]bool, len(out))
	spent := 0

	for _, i := range order {
		if spent >= shiftBudget {
			break
		}
		if resolved[i] {
			continue
		}
		before, after := at(out[i].Peak-beforeMargin/fps), at(out[i].Peak+afterMargin/fps)
		if before == nil || after == nil {
			continue
		}
		spent++

		// Try this event with each partner that began at the same instant,
		// widest first, then alone.
		partner := -1
		region := rect(out[i].Region)
		for _, j := range order {
			if j == i || resolved[j] || out[j].Peak != out[i].Peak {
				continue
			}
			union := region.Union(rect(out[j].Region))
			if area(union) < 2*max(area(region), area(rect(out[j].Region))) {
				continue
			}
			if moved := measureShift(before, after, union, opt); moved.Moved() {
				out[i] = asShift(out[i], union, moved, opt)
				resolved[i], resolved[j] = true, true
				partner = j
				break
			}
		}
		if partner >= 0 {
			continue
		}
		if moved := measureShift(before, after, region, opt); moved.Moved() {
			out[i] = asShift(out[i], region, moved, opt)
			resolved[i] = true
		}
	}

	markOngoingShifts(out, opt)

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
func resolveWholeFrame(events []Event, opt TimelineOptions, fps float64, at func(float64) image.Image) {
	whole := image.Rect(0, 0, opt.SourceWidth, opt.SourceHeight)
	for i := range events {
		// Cuts, and any whole-frame stretch of activity. A real overlay usually
		// animates: a modal backdrop fading in over a third of a second is a
		// run of transitions, not the single one a cut is, and testing only
		// cuts missed the commonest case there is. A flash is excluded because
		// it returns to what it was, so "did the content change or only its
		// brightness" has no meaning for one.
		animated := events[i].Kind == KindBusy && events[i].RegionArea > frameWideArea
		if events[i].Kind != KindCut && !animated {
			continue
		}
		// Straddle the whole stretch, not just its peak: for an animated fade
		// the midpoint is halfway dimmed and compares against nothing useful.
		// The event's bounds are where activity crossed the noise floor, and a
		// fade is still finishing on either side of them, so the sample is
		// taken clear of the transition rather than at its edge.
		before, after := at(events[i].Start-settleMargin/fps), at(events[i].End+settleMargin/fps)
		if before == nil || after == nil {
			continue
		}
		fit, scale, uniform := uniformShade(before, after, whole)
		events[i].ShadeFit, events[i].ShadeScale, events[i].Uniform = fit, scale, uniform
		events[i].Summary = wholeFrameSummary(events[i])
	}
}

// markOngoingShifts flags a translation that is one step of something already
// moving, rather than a discrete layout shift.
//
// A news ticker sliding two pixels at a time is a real translation every time,
// and on a real Forbes recording it produced seven separate "layout shifts" in
// one horizontal strip. They are all true and none of them is the fault anyone
// is looking for: a layout shift is a one-off, and a marquee is not. The tell
// is that the region is already busy for most of the recording.
func markOngoingShifts(events []Event, opt TimelineOptions) {
	for i := range events {
		if events[i].Kind != KindShift {
			continue
		}
		for j := range events {
			if i == j || !longRunning(events[j], opt) {
				continue
			}
			if !overlapping(events[i].Region, events[j].Region) {
				continue
			}
			if events[i].Peak < events[j].Start || events[i].Peak > events[j].End {
				continue
			}
			events[i].Continuous = true
			events[i].Summary += " It sits inside activity that runs in the same place for most of the recording, so it is one step of something moving continuously rather than a one-off layout shift."
			break
		}
	}
}

// longRunning reports an event that occupies enough of the interval to be the
// backdrop other events happen against.
func longRunning(e Event, opt TimelineOptions) bool {
	return e.Kind != KindShift && opt.Span > 0 && (e.End-e.Start)/opt.Span >= continuousShare
}

func measureShift(before, after image.Image, region image.Rectangle, opt TimelineOptions) Displacement {
	span := max(opt.SourceWidth, opt.SourceHeight)
	limit := min(max(region.Dx(), region.Dy())/2, span/3)
	moved := Translation(before, after, region, limit)
	if moved.Moved() && !explainsChange(before, after, region, moved) {
		return Displacement{}
	}
	return moved
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
// brieflyChangedOrder picks the events worth spending frames on.
//
// A layout shift usually animates. An accordion expanding, a disclosure
// widget, anything with a CSS transition on it moves its content over several
// frames, which makes it a stretch of activity rather than the single step a
// shift was first assumed to be — so the commonest shift on a real page could
// not be reported at all. Sustained activity is a candidate too, unless it
// runs long enough to be the backdrop the recording happens against, which is
// a ticker or a spinner rather than a layout settling.
//
// An overlay is excluded: content whose brightness changed under a scrim is
// not content that moved, and asking would only invite a coincidence.
func brieflyChangedOrder(events []Event, opt TimelineOptions) []int {
	var order []int
	for i, e := range events {
		if e.Uniform {
			continue
		}
		if e.Kind == KindStep || e.Kind == KindBlip || (e.Kind == KindBusy && !longRunning(e, opt)) {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool {
		return events[order[a]].Prominence() > events[order[b]].Prominence()
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
