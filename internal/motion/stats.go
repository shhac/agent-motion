package motion

import (
	"math"
	"sort"
)

// Prominence ranks events by how much they changed, on whichever timescale saw
// them. Every list an agent reads must agree on this, or two of them are
// ordered by different rules — the trimmed event list, the suggested
// inspection timestamps, the events worth fetching frames for, and the sweep
// engine proposes. It is a method rather than a package function because the
// fifth of those lives in another package, where a private copy of the formula
// went on compiling and silently disagreeing.
func (e Event) Prominence() float64 { return math.Max(e.PeakChanged, e.PeakDrift) }

// spread is the standard deviation of a series: how much signal there is to
// measure against. Two calibrated thresholds are stated against it —
// minProfileSpread, below which a brightness profile has nothing to register a
// displacement against, and minShadeVariation, below which a frame is too flat
// to tell one brightness map from another. They threshold the same quantity,
// which two copies under two names hid.
func spread(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum, sumSq float64
	for _, v := range values {
		sum += v
		sumSq += v * v
	}
	n := float64(len(values))
	return math.Sqrt(math.Max(0, sumSq/n-(sum/n)*(sum/n)))
}

func round2(v float64) float64 { return roundTo(v, 2) }

func round3(v float64) float64 { return roundTo(v, 3) }
func round4(v float64) float64 { return roundTo(v, 4) }

func roundTo(v float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(v*scale) / scale
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

// persisted and reverted are deliberately not named after truthiness: a nil
// Persists means "could not be compared", and in the borrowed vocabulary that
// would read as reverted, which is the opposite of what it means.
func persisted(b *bool) bool { return b != nil && *b }

func reverted(b *bool) bool { return b != nil && !*b }
