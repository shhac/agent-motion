// Package motion turns a stream of decoded frames into per-transition
// statistics, per-pixel accumulations, and a described timeline. Nothing here
// runs a process or touches the filesystem, so every behaviour is testable
// from synthetic frames.
package motion

import (
	"image"
	"math"

	"github.com/shhac/agent-motion/internal/video"
	output "github.com/shhac/lib-agent-output"
)

// Options configures one analysis pass.
type Options struct {
	// Threshold is the mean absolute RGB delta a pixel must exceed to count
	// as changed, on a 0..255 scale.
	Threshold float64
	// GridCols and GridRows describe the coarse spatial grid reported per
	// transition. Zero means the default 4x3.
	GridCols, GridRows int
	// Checkpoints bounds how many low-resolution frames are retained so an
	// event can be classified as persistent or transient. Zero disables it.
	Checkpoints int
	// ExpectedFrames sizes the checkpoint stride. Zero keeps every frame
	// until the budget is reached and then thins.
	ExpectedFrames int
	// DriftFrames compares each frame against the frame this many frames
	// earlier, so changes too slow to clear Threshold between adjacent frames
	// are still seen. Zero disables the second timescale.
	DriftFrames int
	// IgnoreAbove keeps transitions that change more than this fraction of the
	// frame out of the per-pixel accumulation. A cut or a flash otherwise
	// saturates every pixel and erases the structure the image exists to show.
	// It never affects the per-transition series. Zero disables it.
	IgnoreAbove float64
}

func (o Options) withDefaults() Options {
	if o.GridCols <= 0 {
		o.GridCols = 4
	}
	if o.GridRows <= 0 {
		o.GridRows = 3
	}
	return o
}

// Sample is one frame-to-frame transition. Coordinates are in analysis pixels.
type Sample struct {
	Index int     `json:"index"`
	Time  float64 `json:"time_seconds"`
	// Changed is the fraction of pixels differing from the previous frame.
	Changed float64 `json:"changed_fraction"`
	// Drift is the fraction differing from the frame DriftFrames earlier. It
	// is the only signal that sees a fade or any change slower than the
	// threshold per frame.
	Drift  float64         `json:"drift_fraction"`
	Energy float64         `json:"energy"`
	BBox   image.Rectangle `json:"-"`
	// DriftBBox bounds the pixels differing from the delayed frame.
	DriftBBox image.Rectangle `json:"-"`
	CX, CY    float64         `json:"-"`
	Cells     []float64       `json:"-"`
}

// PixelStats are the per-pixel accumulations behind the projection image.
type PixelStats struct {
	Width, Height int
	Magnitude     []float64
	WeightedTime  []float64
	Changes       []int32
	Reversals     []int32
	Start, End    float64
}

// Analyzer consumes frames in order and accumulates everything the timeline
// and the projection need from a single decode pass.
type Analyzer struct {
	opt           Options
	width, height int
	pixels        int

	previous []byte
	first    []byte
	last     []byte

	magnitude    []float64
	weightedTime []float64
	changes      []int32
	reversals    []int32
	lastSign     []int8

	deltas       []float32
	changedIndex []int32
	ignored      []float64

	samples     []Sample
	accumulated int
	frames      int
	start       float64
	end         float64

	checkpoints []checkpoint
	cpWidth     int
	cpHeight    int
	cpStride    int
	cpCounter   int

	ring     [][]byte
	ringSize int
	pushed   int
	lag      int
}

type checkpoint struct {
	time float64
	pix  []byte
}

// New returns an Analyzer for frames of the given analysis size.
func New(width, height int, opt Options) *Analyzer {
	opt = opt.withDefaults()
	pixels := width * height
	a := &Analyzer{
		opt: opt, width: width, height: height, pixels: pixels,
		magnitude:    make([]float64, pixels),
		weightedTime: make([]float64, pixels),
		changes:      make([]int32, pixels),
		reversals:    make([]int32, pixels),
		lastSign:     make([]int8, pixels),
		deltas:       make([]float32, pixels),
		changedIndex: make([]int32, 0, pixels),
	}
	if opt.DriftFrames > 0 {
		// Two spare slots let drift compare against two references, so a
		// single anomalous frame cannot masquerade as a slow change.
		a.lag = opt.DriftFrames
		a.ringSize = opt.DriftFrames + driftSpare + 1
		a.ring = make([][]byte, a.ringSize)
		for i := range a.ring {
			a.ring[i] = make([]byte, pixels*3)
		}
	}
	if opt.Checkpoints > 0 {
		a.cpWidth, a.cpHeight = checkpointSize(width, height)
		a.cpStride = 1
		if opt.ExpectedFrames > opt.Checkpoints {
			a.cpStride = opt.ExpectedFrames / opt.Checkpoints
		}
	}
	return a
}

func checkpointSize(width, height int) (int, int) {
	const target = 96
	if width <= target {
		return width, height
	}
	h := int(math.Round(float64(height) * target / float64(width)))
	if h < 1 {
		h = 1
	}
	return target, h
}

// Add folds one frame into the accumulation. Frames must arrive in time order.
func (a *Analyzer) Add(f video.Frame) error {
	if len(f.Pix) != a.pixels*3 {
		return output.New("decoded frame has an unexpected pixel count", output.FixableByRetry).
			WithHint("the decoder returned a frame size the analyser did not request")
	}
	if a.frames == 0 {
		a.previous = append([]byte(nil), f.Pix...)
		a.first = append([]byte(nil), f.Pix...)
		a.last = append([]byte(nil), f.Pix...)
		a.start, a.end = f.Time, f.Time
		a.frames = 1
		a.recordCheckpoint(f)
		a.pushDelay(f.Pix)
		return nil
	}
	sample := a.difference(f)
	a.drift(f, &sample)
	a.samples = append(a.samples, sample)
	copy(a.previous, f.Pix)
	copy(a.last, f.Pix)
	a.frames++
	a.end = f.Time
	a.recordCheckpoint(f)
	a.pushDelay(f.Pix)
	return nil
}

// driftSpare is how much older the second drift reference is. A transient that
// lasts fewer frames than this cannot appear in both references.
const driftSpare = 2

// pushDelay stores the frame in the ring used for the slow timescale.
func (a *Analyzer) pushDelay(pix []byte) {
	if a.ringSize == 0 {
		return
	}
	copy(a.ring[a.pushed%a.ringSize], pix)
	a.pushed++
}

// frameAt returns retained frame number i, or nil once it has been overwritten.
func (a *Analyzer) frameAt(i int) []byte {
	if i < 0 || i >= a.pushed || a.pushed-i > a.ringSize {
		return nil
	}
	return a.ring[i%a.ringSize]
}

// drift measures change across the slow window. It takes the smaller of two
// references so one odd frame in the past does not read as a slow change.
func (a *Analyzer) drift(f video.Frame, s *Sample) {
	if a.ringSize == 0 {
		return
	}
	near := a.frameAt(a.pushed - a.lag)
	far := a.frameAt(a.pushed - a.lag - driftSpare)
	if near == nil || far == nil {
		return
	}
	nearCount, nearBox := a.compare(f.Pix, near)
	farCount, farBox := a.compare(f.Pix, far)
	count, box := nearCount, nearBox
	if farCount < nearCount {
		count, box = farCount, farBox
	}
	s.Drift = float64(count) / float64(a.pixels)
	s.DriftBBox = box
}

// compare counts pixels differing from reference by more than the threshold.
func (a *Analyzer) compare(current, reference []byte) (int, image.Rectangle) {
	changed := 0
	minX, minY, maxX, maxY := a.width, a.height, -1, -1
	for p := 0; p < a.pixels; p++ {
		i := p * 3
		delta := (absDiff(current[i], reference[i]) +
			absDiff(current[i+1], reference[i+1]) +
			absDiff(current[i+2], reference[i+2])) / 3
		if delta <= a.opt.Threshold {
			continue
		}
		changed++
		x, y := p%a.width, p/a.width
		minX, minY = min(minX, x), min(minY, y)
		maxX, maxY = max(maxX, x), max(maxY, y)
	}
	if maxX < 0 {
		return 0, image.Rectangle{}
	}
	return changed, image.Rect(minX, minY, maxX+1, maxY+1)
}

func (a *Analyzer) difference(f video.Frame) Sample {
	cols, rows := a.opt.GridCols, a.opt.GridRows
	cells := make([]float64, cols*rows)
	var energy, weightX, weightY, weight float64
	minX, minY, maxX, maxY := a.width, a.height, -1, -1
	a.changedIndex = a.changedIndex[:0]

	for p := 0; p < a.pixels; p++ {
		i := p * 3
		delta := (absDiff(f.Pix[i], a.previous[i]) +
			absDiff(f.Pix[i+1], a.previous[i+1]) +
			absDiff(f.Pix[i+2], a.previous[i+2])) / 3
		energy += delta
		if delta <= a.opt.Threshold {
			continue
		}
		a.deltas[p] = float32(delta)
		a.changedIndex = append(a.changedIndex, int32(p))

		x, y := p%a.width, p/a.width
		cells[(y*rows/a.height)*cols+(x*cols/a.width)]++
		weightX += float64(x) * delta
		weightY += float64(y) * delta
		weight += delta
		minX, minY = min(minX, x), min(minY, y)
		maxX, maxY = max(maxX, x), max(maxY, y)
	}

	changed := len(a.changedIndex)
	cellPixels := float64(a.pixels) / float64(cols*rows)
	for i := range cells {
		cells[i] /= cellPixels
	}
	s := Sample{
		Index: len(a.samples), Time: f.Time,
		Changed: float64(changed) / float64(a.pixels),
		Energy:  energy / float64(a.pixels),
		Cells:   cells,
	}
	if maxX >= 0 {
		s.BBox = image.Rect(minX, minY, maxX+1, maxY+1)
		s.CX, s.CY = weightX/weight, weightY/weight
	}
	a.accumulate(f, s)
	return s
}

// accumulate folds one transition into the per-pixel statistics behind the
// image. Transitions that rewrite most of the frame are recorded and skipped:
// keeping them would flatten every other region to noise.
func (a *Analyzer) accumulate(f video.Frame, s Sample) {
	if a.opt.IgnoreAbove > 0 && s.Changed > a.opt.IgnoreAbove {
		a.ignored = append(a.ignored, s.Time)
		return
	}
	a.accumulated++
	for _, idx := range a.changedIndex {
		p := int(idx)
		i := p * 3
		delta := float64(a.deltas[p])
		a.magnitude[p] += delta
		a.weightedTime[p] += delta * f.Time
		a.changes[p]++
		sign := int8(1)
		if int(f.Pix[i])+int(f.Pix[i+1])+int(f.Pix[i+2]) <
			int(a.previous[i])+int(a.previous[i+1])+int(a.previous[i+2]) {
			sign = -1
		}
		if a.lastSign[p] != 0 && sign != a.lastSign[p] {
			a.reversals[p]++
		}
		a.lastSign[p] = sign
	}
}

func (a *Analyzer) recordCheckpoint(f video.Frame) {
	if a.opt.Checkpoints <= 0 {
		return
	}
	if a.cpCounter%a.cpStride != 0 {
		a.cpCounter++
		return
	}
	a.cpCounter++
	if len(a.checkpoints) >= a.opt.Checkpoints {
		a.thinCheckpoints()
	}
	pix := make([]byte, a.cpWidth*a.cpHeight*3)
	downsample(f.Pix, a.width, a.height, pix, a.cpWidth, a.cpHeight)
	a.checkpoints = append(a.checkpoints, checkpoint{time: f.Time, pix: pix})
}

// thinCheckpoints halves the retained set and doubles the stride, keeping a
// bounded, evenly spaced history for a stream of unknown length.
func (a *Analyzer) thinCheckpoints() {
	kept := a.checkpoints[:0]
	for i, c := range a.checkpoints {
		if i%2 == 0 {
			kept = append(kept, c)
		}
	}
	a.checkpoints = kept
	a.cpStride *= 2
}

// Frames is the number of frames folded in.
func (a *Analyzer) Frames() int { return a.frames }

// Accumulated is the number of transitions that contributed to the image.
func (a *Analyzer) Accumulated() int { return a.accumulated }

// Ignored lists the timestamps of transitions excluded from the image by
// Options.IgnoreAbove.
func (a *Analyzer) Ignored() []float64 { return a.ignored }

// Span returns the first and last frame timestamps seen.
func (a *Analyzer) Span() (float64, float64) { return a.start, a.end }

// Samples returns the per-transition series.
func (a *Analyzer) Samples() []Sample { return a.samples }

// Pixels returns the per-pixel accumulations for image rendering.
func (a *Analyzer) Pixels() PixelStats {
	return PixelStats{
		Width: a.width, Height: a.height,
		Magnitude: a.magnitude, WeightedTime: a.weightedTime,
		Changes: a.changes, Reversals: a.reversals,
		Start: a.start, End: a.end,
	}
}

// Coverage is the fraction of pixels that changed at least once.
func (a *Analyzer) Coverage() float64 {
	active := 0
	for _, c := range a.changes {
		if c > 0 {
			active++
		}
	}
	return float64(active) / float64(a.pixels)
}

func absDiff(a, b byte) float64 {
	if a > b {
		return float64(a - b)
	}
	return float64(b - a)
}

func downsample(src []byte, sw, sh int, dst []byte, dw, dh int) {
	for y := 0; y < dh; y++ {
		sy := y * sh / dh
		for x := 0; x < dw; x++ {
			sx := x * sw / dw
			s, d := (sy*sw+sx)*3, (y*dw+x)*3
			dst[d], dst[d+1], dst[d+2] = src[s], src[s+1], src[s+2]
		}
	}
}
