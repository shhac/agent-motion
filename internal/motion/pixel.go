package motion

// pixelDelta is the mean absolute RGB difference at one pixel offset. It is the
// definition --threshold is measured against, so it lives in exactly one place:
// three verbatim copies meant a change to the metric could silently miss a path.
func pixelDelta(a, b []byte, i int) float64 {
	return (absDiff(a[i], b[i]) + absDiff(a[i+1], b[i+1]) + absDiff(a[i+2], b[i+2])) / 3
}

func absDiff(a, b byte) float64 {
	if a > b {
		return float64(a - b)
	}
	return float64(b - a)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func minI16(a, b int16) int16 {
	if a < b {
		return a
	}
	return b
}

func maxI16(a, b int16) int16 {
	if a > b {
		return a
	}
	return b
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
