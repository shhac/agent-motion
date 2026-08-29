# Architecture

```text
cmd/agent-motion
  └─ internal/cli          Cobra command tree, shared flags, output formatting
       └─ internal/engine  Composes a pass: decode → analyse → describe → render
            ├─ internal/video    Decoder interface; FFmpeg implementation; fake
            ├─ internal/motion   Pure analysis: accumulation, segmentation, prose
            └─ internal/render   Pure image production: activity map, sheet, text
  internal/fixture         A synthetic scenario with exactly known events
  tools/genfixture         Renders that scenario to an MP4 for evaluation
```

## The decoder seam

`internal/video` owns every external process. Nothing else in the tree runs a
command or reads a video file.

```go
type Decoder interface {
    Probe(ctx context.Context, path string) (Info, error)
    Decode(ctx context.Context, req Request, fn func(Frame) error) error
    Still(ctx context.Context, path string, at float64, width int) ([]byte, error)
}
```

`engine.Engine` holds a `Decoder`, so the CLI wires `video.FFmpeg` and tests
wire `video.Fake`. `Fake` replays `internal/fixture` and honours the interval,
rate and scale in the request, which is what makes command-level tests
meaningful rather than decorative. The whole suite runs with no video file and
no FFmpeg installed.

`Request` always carries explicit `Width`, `Height` and `FPS`. The decoder pins
both with FFmpeg's `fps` and `scale` filters, so the raw stream has a known
shape and frame index maps to a known timestamp. A consumer never has to guess.

## The analysis core

`internal/motion` consumes frames and produces statistics and prose. It imports
no decoder and touches no filesystem, so its behaviour is defined entirely by
the frames it is given.

One decode pass produces everything:

- **Per transition**: changed fraction, energy, bounding box, activity centroid,
  coarse grid — the series that segmentation runs on.
- **Across the slow window**: the same comparison against a delayed frame.
- **Per pixel**: accumulated magnitude, change-weighted time, change count and
  sign reversals — the four numbers the activity image is drawn from.
- **Checkpoints**: a bounded set of low-resolution frames, used to answer
  whether a region still looks different after an event.

Segmentation, classification and narration then run over the series in memory.

## Memory

Per-pixel state is `O(width × height)`, independent of length. The slow
timescale holds a ring of `drift × fps` frames at analysis resolution; at the
320px default that is a few megabytes. Checkpoints are capped at 128 frames at
96px wide and thin themselves by doubling their stride, so an unbounded stream
stays bounded. The p99 normalisation for the image sorts active-pixel
magnitudes, which is `O(active pixels)`.

## Why cuts are excluded from the image

A transition that rewrites most of the frame contributes enormous magnitude to
every pixel at once, which flattens every other region to noise — measured, not
assumed. `motion.Options.IgnoreAbove` keeps such transitions out of the
per-pixel accumulation while leaving the per-transition series untouched, so
they are still reported as events. The excluded timestamps are returned in the
result.

## What is deliberately absent

No credential storage, network client, cache, daemon or MCP surface. None of
these are inherited merely because sibling `agent-*` tools have them.
