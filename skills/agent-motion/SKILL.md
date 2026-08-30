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
| `motion` | activity whose centre travels; `direction` and `travel_pixels` reported |
| `gradual` | too slow to see between frames; found over the `--drift` window |
| `busy` | sustained activity with no clearer shape |
| `stall` | activity that was running continuously stopped, then resumed |

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

## Limits worth stating back

- No object recognition, no text reading, no explanation of cause.
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
