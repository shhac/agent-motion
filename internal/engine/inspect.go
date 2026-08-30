package engine

import (
	"context"
	"fmt"

	"github.com/shhac/agent-motion/internal/video"
)

// Inspection is the cheap first call: what this file is, without decoding it.
type Inspection struct {
	Input     string     `json:"input"`
	Source    video.Info `json:"source"`
	NextSteps []string   `json:"next_steps"`
}

// Inspect reads container and stream metadata only.
func (e *Engine) Inspect(ctx context.Context, path string) (*Inspection, error) {
	info, err := e.Decoder.Probe(ctx, path)
	if err != nil {
		return nil, err
	}
	return &Inspection{
		Input:  path,
		Source: info,
		NextSteps: []string{
			fmt.Sprintf("agent-motion timeline %s", quote(path)),
			fmt.Sprintf("agent-motion sheet %s", quote(path)),
		},
	}, nil
}
