# agent-motion

`agent-motion` creates a temporal projection: one spatially aligned PNG that
encodes which pixels changed over a selected video interval, plus JSON that
explains the encoding to an LLM or agent.

It is aimed at fixed-viewport recordings such as UI captures, visual tests,
browser behaviour, rendering, and game debugging. It does not reconstruct a
video and should not be treated as a general video summary.

> **Status:** early prototype. The first deterministic frame-difference mode
> is implemented; optical flow, camera stabilization, and the LLM evaluation
> harness are intentionally future work. The design record lives in
> [`design-docs/`](design-docs/).

## Why

Video debugging often has a mostly static screen. A frame atlas repeats those
static pixels over and over. A temporal projection keeps the original `x,y`
coordinates and concentrates its image budget on where and when pixels change,
giving an image-capable agent a compact starting point for temporal zoom.

## Requirements

- Go 1.26+
- `ffmpeg` and `ffprobe` on `PATH` (or pass `--ffmpeg` / `--ffprobe`)

## Build and run

```sh
make build
./agent-motion project recording.mp4 --start 12 --end 18
```

This writes `recording.temporal.png` beside the input and prints metadata such
as frame count, source FPS, change coverage, peak activity time, and the exact
RGB-channel meaning. Choose another path with `--output`.

```sh
agent-motion project recording.mp4 --start 17 --end 19 --threshold 12 \
  --output /tmp/recording-17-19.png
```

Use progressively smaller intervals to investigate suspicious activity:

```text
0–30s projection → 17–19s projection → 17.5–17.9s projection → frames
```

Run `agent-motion usage` or `agent-motion project usage` for the agent-facing
contract.

## v1 encoding

For each pixel, v1 compares successive RGB frames and suppresses changes below
`--threshold`.

- **red** — accumulated change magnitude, normalized within this projection
- **green** — mean time of detected change (early → dark, late → bright)
- **blue** — change frequency, with extra emphasis for sign reversals

Black means no detected activity. This is an activity map, not a faithful
rendering of the source frame. See the metadata rather than assuming channel
semantics.

## Development

```sh
make test
make vet
make lint
```
