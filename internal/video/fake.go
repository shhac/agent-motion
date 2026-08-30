package video

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
)

// Fake is an in-memory Decoder for tests. It renders frames from a pure
// function of frame index, so analysis and command behaviour can be exercised
// with no FFmpeg installed and no media on disk.
type Fake struct {
	Info Info
	// Render fills dst with one native-resolution rgb24 frame.
	Render func(dst []byte, index int)

	ProbeErr  error
	DecodeErr error
	// StillPNG overrides the rendered still. Left nil, Still renders the real
	// frame at the requested time, which is what makes contact-sheet tests
	// exercise the same content the analysis saw.
	StillPNG []byte

	Requests []Request
	Stills   []float64
}

var _ Decoder = (*Fake)(nil)

// Probe returns the configured Info.
func (f *Fake) Probe(_ context.Context, _ string) (Info, error) {
	if f.ProbeErr != nil {
		return Info{}, f.ProbeErr
	}
	return f.Info, nil
}

// Decode replays the scenario honouring the interval, rate and scale of req,
// matching what FFmpeg's fps and scale filters would produce closely enough to
// exercise callers.
func (f *Fake) Decode(_ context.Context, req Request, fn func(Frame) error) error {
	f.Requests = append(f.Requests, req)
	if f.DecodeErr != nil {
		return f.DecodeErr
	}
	end := req.End
	if end <= req.Start {
		end = f.Info.Duration
	}
	native := make([]byte, f.Info.Width*f.Info.Height*3)
	frame := Frame{Time: req.Start, Pix: make([]byte, req.Width*req.Height*3)}
	count := int(math.Floor((end - req.Start) * req.FPS))
	for i := 0; i < count; i++ {
		t := req.Start + float64(i)/req.FPS
		source := int(math.Floor(t * f.Info.FPS))
		if source >= f.Info.NBFrames && f.Info.NBFrames > 0 {
			break
		}
		f.Render(native, source)
		scale(native, f.Info.Width, f.Info.Height, frame.Pix, req.Width, req.Height)
		frame.Index, frame.Time = i, t
		if err := fn(frame); err != nil {
			return err
		}
	}
	return nil
}

// Still records the request and renders the frame at that timestamp.
func (f *Fake) Still(_ context.Context, _ string, still Still) ([]byte, error) {
	f.Stills = append(f.Stills, still.At)
	if f.StillPNG != nil || f.Render == nil {
		return f.StillPNG, nil
	}
	native := make([]byte, f.Info.Width*f.Info.Height*3)
	f.Render(native, int(math.Floor(still.At*f.Info.FPS)))
	src, srcW, srcH := crop(native, f.Info.Width, f.Info.Height, still.Crop)

	w, h := srcW, srcH
	if still.Width > 0 {
		w = still.Width
		h = max(2, even(int(math.Round(float64(srcH)*float64(w)/float64(srcW)))))
	}
	small := make([]byte, w*h*3)
	scale(src, srcW, srcH, small, w, h)

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for p := 0; p < w*h; p++ {
		i := p * 3
		img.SetRGBA(p%w, p/w, color.RGBA{R: small[i], G: small[i+1], B: small[i+2], A: 0xff})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// crop returns the requested rectangle of a frame, or the frame itself.
func crop(src []byte, w, h int, r image.Rectangle) ([]byte, int, int) {
	r = r.Intersect(image.Rect(0, 0, w, h))
	if r.Empty() {
		return src, w, h
	}
	out := make([]byte, r.Dx()*r.Dy()*3)
	for y := 0; y < r.Dy(); y++ {
		from := ((y+r.Min.Y)*w + r.Min.X) * 3
		copy(out[y*r.Dx()*3:(y+1)*r.Dx()*3], src[from:from+r.Dx()*3])
	}
	return out, r.Dx(), r.Dy()
}

// scale is nearest neighbour, matching the fake's purpose of shape fidelity
// rather than image quality.
func scale(src []byte, sw, sh int, dst []byte, dw, dh int) {
	if sw == dw && sh == dh {
		copy(dst, src)
		return
	}
	for y := 0; y < dh; y++ {
		sy := y * sh / dh
		for x := 0; x < dw; x++ {
			sx := x * sw / dw
			s, d := (sy*sw+sx)*3, (y*dw+x)*3
			dst[d], dst[d+1], dst[d+2] = src[s], src[s+1], src[s+2]
		}
	}
}
