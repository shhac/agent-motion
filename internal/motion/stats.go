package motion

import (
	"math"
	"sort"
)

// prominence ranks events by how much they changed, on whichever timescale saw
// them. Both the trimmed event list and the suggested inspection timestamps
// must agree on this, or an agent reads two lists ordered by different rules.
func prominence(e Event) float64 { return math.Max(e.PeakChanged, e.PeakDrift) }

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

func truthy(b *bool) bool { return b != nil && *b }

func falsey(b *bool) bool { return b != nil && !*b }
