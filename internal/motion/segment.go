package motion

import (
	"image"
	"math"
	"sort"
)

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
