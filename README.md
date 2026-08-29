# agent-motion

Find out what happens in a video over time, without watching it.

`agent-motion` decodes a recording locally and returns a described timeline:
what changed, when, where on screen, and what shape the change had. It writes
images when you need to see something, and it tells you what it could not have
seen.

Built for AI agents, useful at a terminal.

```console
$ agent-motion timeline recording.mp4
```

```text
Analysed 0.00s to 27.97s. Found 2 hard cuts, 1 movement, 1 one-off change that
persists, 1 repeated toggle, 1 whole-frame flash and 1 gradual change. The
busiest moment is 15.00s.

motion   2.00-5.00   Movement from 2.00s to 5.00s in the middle centre; the
                     active area travels left to right across about 454 px.
step     6.50        One-off change at 6.50s in the bottom right (60x24 px at
                     500,300) that is still there afterwards.
flicker  9.00-11.90  Repeated toggling in the top centre, about 10.3 changes
                     per second over 2.90s.
cut      15.00       Hard cut: 100% of the frame changes in one transition.
flash    21.00       Whole-frame flash lasting about 30 ms.
gradual  22.47-27.60 Gradual change in the bottom centre. Too slow to clear the
                     threshold between adjacent frames.
```

## Why

Sampling frames into an atlas spends most of its budget re-reading a static
screen, and loses whatever happened between two samples. The question is
usually temporal — *when did this break, and where?* — and that answer is
mostly text, at a fraction of the cost of the frames it summarises.

## Install

```sh
brew install shhac/tap/agent-motion
```

Or build it:

```sh
make build
```

Requires Go 1.26+ and `ffmpeg` / `ffprobe` on `PATH` (or pass `--ffmpeg` /
`--ffprobe`).

## Commands

| Command | Cost | What you get |
|---|---|---|
| `inspect` | none | Dimensions, frame rate, duration, codec. No decoding. |
| `timeline` | one pass | The described timeline. Start here. |
| `sheet` | one pass + stills | One PNG of many labelled real frames. |
| `project` | one full-res pass | The timeline plus an activity-map PNG. |
| `frames` | one still each | Real source frames at chosen timestamps. |

Every result carries `next_steps` with commands you can run verbatim, and
`limits` with what that run could not have seen.

```sh
agent-motion sheet recording.mp4                       # see the whole thing
agent-motion timeline recording.mp4 --start 17 --end 19 --threshold 4
agent-motion frames recording.mp4 --at 17.62
```

## What it is good at, and what it is not

Good at **fixed-viewport** recordings: screen captures, browser sessions,
visual regression runs, rendering and game debugging. It finds where and when
pixels changed, including changes far too slow to see between adjacent frames.

It does not recognise objects, read text, or explain why anything changed.
Regions are bounding boxes of change, not outlines. A handheld or panning
camera makes everything change at once, and the results become much weaker.

## The activity image

`project` writes a PNG in which every pixel keeps its source `x,y`: red is how
much it changed, green is when (black early, bright late), blue is how often.
Black is no change. It is an activity map, not a picture of the video — the
exact encoding comes back in the result, and you should read it rather than
infer it from the colours.

## Development

```sh
make test    # runs with no FFmpeg and no media, against a synthetic scenario
make vet
make lint
make fixture # renders the reference video used for evaluation
```

The design record is in [`design-docs/`](design-docs/), including a
[decision log](design-docs/decisions.md) of what was changed and what forced it.

## Licence

[PolyForm Perimeter 1.0.0](LICENSE).
