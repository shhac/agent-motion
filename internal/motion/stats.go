package motion

import (
	"math"
	"sort"
)

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

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

func truthy(b *bool) bool { return b != nil && *b }

func falsey(b *bool) bool { return b != nil && !*b }
