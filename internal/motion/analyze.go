// Package motion turns a stream of decoded frames into per-transition
// statistics, per-pixel accumulations, and a described timeline. Nothing here
// runs a process or touches the filesystem, so every behaviour is defined by
// the frames it is given and testable from synthetic ones.
package motion

import (
	"math"

	"github.com/shhac/agent-motion/internal/video"
	output "github.com/shhac/lib-agent-output"
)

// Options configures one analysis pass.
type Options struct {
	// Threshold is the mean absolute RGB delta a pixel must exceed to count
	// as changed, on a 0..255 scale.
	Threshold float64
	// GridCols and GridRows describe the spatial grid. Events are found per
	// cell and then merged, so two things happening at once in different
	// places stay two events. Zero means the default 8x6.
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
		o.GridCols = 8
	}
	if o.GridRows <= 0 {
		o.GridRows = 6
	}
	return o
}

// Sample is one frame-to-frame transition.
type Sample struct {
	Index int     `json:"index"`
	Time  float64 `json:"time_seconds"`
	// Changed is the fraction of the whole frame differing from the previous
	// frame, which is what identifies a cut or a flash.
	Changed float64 `json:"changed_fraction"`
	// Drift is the same measure against the frame DriftFrames earlier. It is
	// the only signal that sees a fade.
	Drift float64 `json:"drift_fraction"`
	Cells []Cell  `json:"-"`
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
	grid          Grid
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
	nearCells    []Cell
	farCells     []Cell

	samples     []Sample
	ignored     []float64
	accumulated int
	frames      int
	start, end  float64

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

// New returns an Analyzer for frames of the given analysis size.
func New(width, height int, opt Options) *Analyzer {
	opt = opt.withDefaults()
	pixels := width * height
	a := &Analyzer{
		opt: opt, width: width, height: height, pixels: pixels,
		grid:         newGrid(opt.GridCols, opt.GridRows, width, height),
		magnitude:    make([]float64, pixels),
		weightedTime: make([]float64, pixels),
		changes:      make([]int32, pixels),
		reversals:    make([]int32, pixels),
		lastSign:     make([]int8, pixels),
		deltas:       make([]float32, pixels),
		changedIndex: make([]int32, 0, pixels),
	}
	if opt.DriftFrames > 0 {
		a.nearCells = make([]Cell, opt.GridCols*opt.GridRows)
		a.farCells = make([]Cell, opt.GridCols*opt.GridRows)
		// Two spare slots let drift compare against two references, so a
		// single anomalous frame cannot masquerade as a slow change.
		a.lag = boundedLag(opt.DriftFrames, pixels)
		a.ringSize = a.lag + driftSpare + 1
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
	a.accumulate(f, sample)
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

func (a *Analyzer) difference(f video.Frame) Sample {
	cells := make([]Cell, len(a.grid.Pixels))
	resetBounds(cells)
	a.changedIndex = a.changedIndex[:0]

	for p := 0; p < a.pixels; p++ {
		i := p * 3
		delta := pixelDelta(f.Pix, a.previous, i)
		if delta <= a.opt.Threshold {
			continue
		}
		a.deltas[p] = float32(delta)
		a.changedIndex = append(a.changedIndex, int32(p))
		a.mark(cells, p)
	}

	s := Sample{
		Index: len(a.samples), Time: f.Time,
		Changed: float64(len(a.changedIndex)) / float64(a.pixels),
		Cells:   cells,
	}
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

// driftBudget bounds the frames held for the slow timescale. The ring is by far
// the largest allocation: a one-second window on 1920x1080 at 60fps is 373MB of
// retained frames, which is not a reasonable thing for a CLI to do to a machine
// without saying so.
const driftBudget = 192 << 20

// boundedLag shortens the slow window when a full one would not fit in the
// budget. The caller is told the window it actually got, because a shorter one
// sees less gradual change and a result that quietly analysed half of what was
// asked for is worse than one that says it did.
func boundedLag(want, pixels int) int {
	frame := pixels * 3
	if frame <= 0 {
		return want
	}
	return max(2, min(want, driftBudget/frame))
}

// driftSpare is how much older the second drift reference is. A transient that
// lasts fewer frames than this cannot appear in both references.
const driftSpare = 2

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
	nearCount := a.compare(f.Pix, near, a.nearCells)
	farCount := a.compare(f.Pix, far, a.farCells)
	count, cells := nearCount, a.nearCells
	if farCount < nearCount {
		count, cells = farCount, a.farCells
	}
	s.Drift = float64(count) / float64(a.pixels)
	for i := range s.Cells {
		// The comparison wrote into the fast slot of a scratch cell; the winner
		// becomes this sample's slow view.
		s.Cells[i].M[slow] = cells[i].M[fast]
	}
}

// resetBounds prepares cells so the first changed pixel sets both bounds.
func resetBounds(cells []Cell) {
	empty := Cell{}
	for t := range empty.M {
		empty.M[t] = measure{MinX: math.MaxInt16, MinY: math.MaxInt16, MaxX: -1, MaxY: -1}
	}
	for i := range cells {
		cells[i] = empty
	}
}

// compare counts and bounds pixels differing from reference by more than the
// threshold, per cell and in total. Results land in the Changed/Min/Max fields
// of cells; the caller moves whichever reference won into the drift fields.
func (a *Analyzer) compare(current, reference []byte, cells []Cell) int {
	resetBounds(cells)
	changed := 0
	for p := 0; p < a.pixels; p++ {
		i := p * 3
		if pixelDelta(current, reference, i) <= a.opt.Threshold {
			continue
		}
		changed++
		a.mark(cells, p)
	}
	return changed
}

// mark records a changed pixel against its grid cell. The index arithmetic has
// to agree with Grid.Bounds for regions to be right, so it is written once.
// It always writes the fast slot; the drift pass moves its result across.
func (a *Analyzer) mark(cells []Cell, p int) {
	x, y := p%a.width, p/a.width
	m := &cells[a.grid.Cell(x, y)].M[fast]
	m.Count++
	m.MinX, m.MinY = min(m.MinX, int16(x)), min(m.MinY, int16(y))
	m.MaxX, m.MaxY = max(m.MaxX, int16(x)), max(m.MaxY, int16(y))
}

// Frames is the number of frames folded in.
func (a *Analyzer) Frames() int { return a.frames }

// DriftFrames is the slow window actually used, which may be shorter than the
// one requested if a full one would not fit in the memory budget.
func (a *Analyzer) DriftFrames() int { return a.lag }

// Accumulated is the number of transitions that contributed to the image.
func (a *Analyzer) Accumulated() int { return a.accumulated }

// Ignored lists the timestamps of transitions excluded from the image.
func (a *Analyzer) Ignored() []float64 { return a.ignored }

// Span returns the first and last frame timestamps seen.
func (a *Analyzer) Span() (float64, float64) { return a.start, a.end }

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
