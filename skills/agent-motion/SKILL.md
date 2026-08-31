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

`agent-motion inspect <video>` is the cheap first call if you only need
dimensions, frame rate, duration and codec — it decodes nothing.
`agent-motion mcp` serves every command over MCP for a client without a shell.

## Which command answers which question

| Question | Command |
|---|---|
| What happens, and when? | `timeline` |
| **Where** on screen did it happen? | `activity` — NDJSON, one line per busy region |
| What does it actually look like? | `sheet` — one PNG of many captioned real frames |
| What is in this exact frame? | `frames --at 17.62` |
| Are these two moments the same? | `compare --at 14.9,18.5` — exact pixel count |
| Should this build fail? | `check --max-shift-score 0.05` — exits non-zero |
| Where was the action, as a picture? | `project` — activity map PNG |

Only `timeline` and `activity` are needed for most questions. Both take the
same analysis flags, and both carry `limits` and `suitability`.

## Four things not to misread

These are the misreadings that produce confidently wrong answers. Everything
else in this skill is detail; these are load-bearing.

**1. An empty result is not "nothing happened".** Every result carries `limits`
saying what that run could not have seen — a threshold too high, an analysis
width too coarse, a timescale switched off. Read it before reporting an
absence. `activity` returning no cells means nothing happened *in one place
while the rest of the frame held still*; its `frame_wide` meta line is where
whole-frame change is reported.

**2. No `shift` is not "nothing moved".** A displacement is found by
registering one frame against the other, which needs most of the content still
on screen afterwards. A move of more than about half its region — a page
jumping a whole screen, a scroll — leaves too little overlap to measure and is
reported as a `cut` or `busy` change instead. When a large region changes and
nothing claims to have moved, look at the frames.

**3. Kinds name the shape of a change, never its meaning.** A `step` might be a
button appearing, a tooltip closing, or a value updating. The tool does not
recognise objects, read text, or explain cause. `shift` is the one kind that
says what happened to the content — the same pixels in a new place — and it is
only ever set from measurement.

**4. Believe `suitability`.** On footage where everything moves at once the
event list is a list of fragments, and a `check` that passes on it means
nothing. The tool detects this itself and says so.

## Content shift

A `shift` means the pixels that were there are still there, somewhere else —
which on a web page separates a bug from the page working normally.

```json
"kind": "shift", "moved_by_pixels": [0, 40], "layout_shift_score": 0.0275
```

`moved_by_pixels` is the displacement in source pixels, positive Y down,
measured from the two real frames either side rather than from the downscaled
analysis, so it is exact. `layout_shift_score` is the share of the frame
affected times how far it went — CLS-*shaped*, and not Chrome's Cumulative
Layout Shift, which comes from the DOM and knows which elements are unstable.
Use it to rank and threshold, not to report a Core Web Vital.

An animated shift is found as well as an instant one, but one that runs for
most of the recording is marked `continuous` — an animation rather than a
layout settling — and `check` does not count those against a shift limit.

The tool cannot tell you *which element* moved or *why*; that needs the DOM.
For a live page you control, `PerformanceObserver` with `layout-shift` entries
is the better tool. This one is for when you have a recording and not the page.

## Event kinds at a glance

`cut` `flash` `step` `blip` `flicker` `motion` `gradual` `busy` `stall`
`shift` — defined in [interpreting results](references/interpreting.md#event-kinds).

`stall` is the one worth knowing up front: it is an *absence* of change, so no
pixel shows it. Something that had been animating continuously stopped and then
resumed. On a "the page felt janky" report that is usually the answer.

## Output and errors

One JSON object on stdout; `--format json|yaml|jsonl` overrides it. `jsonl`
renders a result's list one record per line with the rest as meta lines, so
`timeline --format jsonl | grep '"kind":"shift"'` works without parsing.

Failures are one JSON object on stderr with `fixable_by`: `agent` means fix the
input or flags, `human` means install or grant something (FFmpeg must be on
`PATH`, or pass `--ffmpeg` / `--ffprobe`), `retry` means try again.

## Going deeper

- [commands](references/commands.md) — every command and every flag
- [interpreting results](references/interpreting.md) — every output field, the
  event-kind table, and the limits worth stating back
- [recipes](references/recipes.md) — watching one event unfold, seeing
  something small, whether the page finished loading, comparing two moments
- [worked examples](references/worked-examples.md) — real measured output
  beside what was actually happening, for the readings that are easy to get
  backwards
- [images](references/images.md) — `sheet` and `project`, and what to do
  instead if you cannot open a PNG
