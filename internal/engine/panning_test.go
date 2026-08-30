package engine_test

import "math/rand"

// panningNoise renders a textured frame that slides every frame — a stand-in
// for footage where the whole picture is in motion.
func panningNoise(w, h int) func([]byte, int) {
	texture := make([]byte, w*h*3)
	noise := rand.New(rand.NewSource(1))
	for i := range texture {
		texture[i] = byte(noise.Intn(256))
	}
	return func(dst []byte, index int) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				sx, sy := (x+index*3)%w, (y+index*2)%h
				s, d := (sy*w+sx)*3, (y*w+x)*3
				copy(dst[d:d+3], texture[s:s+3])
			}
		}
	}
}
