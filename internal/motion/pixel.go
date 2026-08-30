package motion

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
