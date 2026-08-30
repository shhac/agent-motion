---
name: agent-motion
description: |
  Find out what happens in a video over time without watching it. Use when:
  - Debugging a screen recording, UI capture, browser session, or visual test
  - Locating when something appeared, moved, flickered, flashed, or faded
  - Finding the timestamp of a glitch, a rendering artefact, or a layout jump
  - Deciding which frames of a video are worth looking at, before looking
  - Summarising a recording you cannot afford to sample frame by frame
  Triggers: "video", "screen recording", "screencast", "mp4", "mov", "webm", "what happens in this video", "when does", "find the glitch", "flicker", "flashing", "visual regression", "ui recording", "capture", "frame", "timestamp of", "contact sheet", "extract frames", "motion", "agent-motion", "temporal"
allowed-tools: Bash(agent-motion *) Read Grep Glob
---

# Understanding a video with `agent-motion`

`agent-motion` is a CLI on `$PATH`. It answers **what changed, when, and where**
for a video, as JSON you can act on, and it writes images when you need to see
something. It never uploads anything; decoding is local FFmpeg.

It is built for **fixed-viewport** recordings — a screen capture, a browser
session, a visual test, a rendered scene. A handheld or panning camera makes
every pixel change at once, and the results become much weaker.

`agent-motion inspect <video>` is the cheap first call — dimensions, frame rate,
duration and codec, with nothing decoded. `agent-motion mcp` serves every
command over MCP for a client that does not have a shell.

## Start here

```sh
agent-motion timeline recording.mp4
```

Read the fields in this order:

1. `narrative` — one paragraph describing the whole interval. If the recording
   is a poor fit for this tool, the warning is the first thing in it.
2. `events` — each with `kind`, `start_seconds`, `end_seconds`, `region_xyxy`,
   `position`, and a plain-English `summary`.
3. `suitability` — whether this recording is the kind the tool works on. A
   verdict other than `suitable` means most of the frame moves at once, so the
   event boundaries are arbitrary and small events are fragments of one moving
   scene. Look at a `sheet` instead of trusting the list.
4. `limits` — what this run could not have seen. Read it before concluding
   that nothing happened.
5. `next_steps` — commands you can run verbatim.

`activity_sparkline` shows the shape of frame-to-frame activity at a glance,
one character per bucket, least to most active: `_ . : - = + * #`. Its scale is
relative to `activity_sparkline_full_scale`, so it is for orientation, not
measurement, and `gradual` events do not appear in it.

## Then see it

```sh
agent-motion sheet recording.mp4
```

Writes one PNG containing many real frames, each captioned with its timestamp
and the event it belongs to, choosing the moments from the analysis. Open it
with the Read tool. This is usually the fastest way to learn what a recording
is actually of.

For specific moments, pass `--at`:

```sh
agent-motion sheet recording.mp4 --at 3.4,7.1,12.0
agent-motion frames recording.mp4 --at 17.62      # full-size stills
```

## Watching one event unfold

An event's start and end say a panel toggled ten times a second, or a colour
drifted for four seconds. Neither says what the toggle or the drift *looks*
like. Paste the event's own span back in and let the tool space the samples:

```sh
agent-motion sheet recording.mp4 --during 13.07:13.40 --count 10 \
  --region 498,38,582,102 --pad 12 --quick
```

`--during` works on `frames` too. `next_steps` proposes one of these for any
event with internal cadence.

## Seeing something small

A 20x20 indicator or a 2px layout shift is invisible in a full-frame still.
Crop to the region and magnify — `--region` takes an event's `region_xyxy`
verbatim, and cropping happens before scaling, so `--width` enlarges it:

```sh
agent-motion frames recording.mp4 --at 6.2 --region 200,120,202,160 \
  --pad 24 --width 480
```

`--pad` widens the crop so a thin feature is not flush against the edge. It
works on `sheet` too, which then crops every tile the same way — the fastest
way to watch one small element change over time.

## Content shift

A `shift` is the one kind that says *what happened to the content*, not just
that it changed. It means the pixels that were there are still there, somewhere
else — which on a web page separates a bug from the page working normally.

```json
"kind": "shift", "moved_by_pixels": [0, 40], "layout_shift_score": 0.0275
```

`moved_by_pixels` is the displacement in source pixels, positive Y down,
measured from the two real frames either side of the transition rather than
from the downscaled analysis, so it is exact. `layout_shift_score` is the share
of the frame affected times how far it went. It is CLS-*shaped* and is not
Chrome's Cumulative Layout Shift, which comes from the DOM, covers a session
window, and knows which elements are unstable. Use it to rank and to threshold,
not to report a Core Web Vital.

The tool cannot tell you *which element* moved or *why* — that needs the DOM.
For a live page you control, `PerformanceObserver` with `layout-shift` entries
is the better tool. This one is for when you have a recording and not the page.

## Testing a recording

```sh
agent-motion check recording.mp4 --max-shift-score 0.05 --no-stall
```

Turns the analysis into a pass or fail and exits non-zero on failure, so a
visual regression can break a build rather than waiting to be noticed. Every
threshold is opt-in — with none given it asserts nothing and says so, rather
than implying it looked and approved. Each failed assertion names the event
that broke it.

`--max-shift-score`, `--max-shift-pixels`, `--no-shift`, `--no-stall`,
`--no-flicker`, `--quiet`.

If the recording is one the tool cannot judge — a scroll, a pan, ambient motion
— the result says so in `notes`. A pass on footage like that means nothing, and
it will tell you.

## Ask whether something is the same as it was

```sh
agent-motion compare recording.mp4 --at 14.9,18.5
agent-motion compare recording.mp4 --at 6.13,6.23 \
  --region 200,120,202,160 --pad 24 -o jitter.png
```

Every other command compares neighbouring frames. `compare` takes two arbitrary
timestamps and gives an exact pixel count, which answers questions nothing else
can: did the screen come back to the same state after that cut, did the region
really revert, is anything at all different between these two moments. It
distinguishes *identical* from *nothing above the threshold* — the second is
what codec noise looks like.

With `-o` it draws the difference: the later frame dimmed, with everything that
differs lit up. For a change of a pixel or two, this is the only way to see it —
two nearly identical stills cannot be compared by eye.

## Narrow in

Events give you a range; run again inside it with a lower threshold to see what
was too small or too subtle the first time.

```sh
agent-motion timeline recording.mp4 --start 17 --end 19 --threshold 4
```

`--threshold` is the main dial. It is the per-pixel change, 0..255, that is
ignored. The default 12 suppresses compression noise and also hides genuinely
subtle rendering instability, so lowering it is the standard second move.

## Event kinds

| Kind | Means |
|---|---|
| `cut` | most of the frame changed at once and stayed changed |
| `flash` | most of the frame changed for a frame or two, then returned |
| `step` | brief localised change that is still there afterwards |
| `blip` | brief localised change that reverted |
| `flicker` | one area toggling repeatedly; `changes_per_second` is reported |
| `motion` | activity whose centre travels; `direction` and `travel_pixels` reported. If it reverses once, `jump_backwards_pixels` marks where — usually the bug, when the movement itself is expected |
| `gradual` | too slow to see between frames; found over the `--drift` window |
| `busy` | sustained activity with no clearer shape |
| `stall` | activity that was running continuously stopped, then resumed |
| `shift` | the same content in a new place — it moved rather than appearing |

Kinds describe the **shape** of a change, never its meaning. A `step` might be a
button appearing, a tooltip closing, or a value updating — pull the frames.

`stall` is the exception worth understanding: it is an *absence* of change, so
no pixel shows it. It means something that had been animating continuously —
a spinner, a caret, a polling indicator — stopped and then started again. On a
"the page felt janky" report that is usually the answer.

## The activity image

```sh
agent-motion project recording.mp4
```

Returns everything `timeline` returns and additionally writes a PNG where every
pixel keeps its source `x,y`: red is how much it changed, green is when (black
early, bright late), blue is how often. Black is no change above the threshold.

It is an activity map, not a picture of the video, and it is not the whole
story. Whole-frame cuts are left out so they cannot flatten everything else,
`gradual` events barely register in it, and a `stall` cannot be drawn at all.
Everything it omits is named in `omitted_from_image` and printed into the
legend band. Read that before concluding nothing happened somewhere.

Use `sheet` when you want to know what something looks like, and `project` when
you want to know where on screen the action was.

## If you cannot look at images

`sheet`, `project`, `frames` and `compare -o` write PNGs, and this skill assumes
you can open them. If you cannot, say so rather than guessing at their contents,
and lean on the text instead: `timeline` describes every event, and `compare`
answers questions about specific moments numerically — an exact changed-pixel
count, the box those pixels fall in, and whether two frames are identical. That
path is weaker, because nothing in it says *what* a region contains, but it is a
real one and it is honest.

## Limits worth stating back

- No object recognition, no text reading, no explanation of cause.
- Timestamps are frame-scale. At 30fps every one is accurate to about 33ms, and
  seeking snaps to the nearest frame. Do not quote them more precisely, and
  expect a run at a lower `--sample-fps` to move them.
- Regions are bounding boxes of change, not object outlines.
- Analysis is downscaled to `--analysis-width` (320 by default) unless you pass
  `--native`; thin features can be missed.
- A moving camera, a scrolling page, a slow zoom, or ambient motion — wind in
  foliage, water, fire, a crowd, film grain — makes everything an event. The
  tool detects this itself and says so in `suitability`; believe it, and switch
  to `sheet` and `frames`.
- On a still screen, a gap is just a gap. A `stall` is only reported when
  something that *was* running continuously stopped, so a quiet stretch on an
  otherwise static recording is not one.

## Output and errors

One JSON object on stdout; `--format json|yaml|jsonl` overrides it. Failures are
one JSON object on stderr with `fixable_by`: `agent` means fix the input or
flags, `human` means install or grant something (FFmpeg must be on `PATH`, or
pass `--ffmpeg` / `--ffprobe`), `retry` means try again.

Full flag and field reference: [commands](references/commands.md) and
[interpreting results](references/interpreting.md).
