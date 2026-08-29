package video

import (
	"context"
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
	StillPNG  []byte

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

// Still records the request and returns the configured PNG bytes.
func (f *Fake) Still(_ context.Context, _ string, at float64, _ int) ([]byte, error) {
	f.Stills = append(f.Stills, at)
	return f.StillPNG, nil
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
