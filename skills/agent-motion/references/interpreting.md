# Reading `agent-motion` results

## Result fields

| Field | What it is |
|---|---|
| `narrative` | One paragraph covering the whole interval. Read it first. A suitability warning, when there is one, leads it. |
| `suitability` | `verdict` of `suitable`, `marginal` or `unsuitable`, with a reason and advice. `typical_changed_fraction` is the median share of the frame changing per frame. |
| `events` | The described occurrences, in time order. |
| `quiet_ranges` | Stretches with no detected change. Two ranges meeting at one timestamp are separated by an instantaneous event, not joined. |
| `busiest_seconds` | Timestamp of the single largest frame-to-frame change. |
| `settled_at_seconds` | When the last change of any kind finished. Absent when the recording ended mid-change, which `still_changing_at_end` then says. |
| `layout_settled_at_seconds` | When the last change to the *content* finished, ignoring anything that merely keeps animating. This is the one that answers "has this finished loading" — a ticker or a spinner keeps `settled_at_seconds` late while the page itself has been stable for seconds. |
| `activity_sparkline` | Shape of frame-to-frame activity, one character per bucket. Least to most active: `_ . : - = + * #`. `gradual` events do not appear in it. |
| `activity_sparkline_full_scale` | The value a `#` represents. The ramp is square-root scaled, so it is orientation, not measurement. |
| `bucket_seconds` | How much time one sparkline character covers. |
| `activity_by_bucket` | The numbers behind the sparkline, one per character, present only with `--series`. Use it when you need the shape as data rather than as a picture. |
| `motion_coverage` | Fraction of pixels that changed at least once. |
| `timestamps_worth_inspecting` | Frames that would show the events found. |
| `next_steps` | Commands you can run verbatim. |
| `limits` | What this run could not have seen. |
| `analysis` | Exactly how the pass was done, so it can be reproduced. |

## Event fields

| Field | What it is |
|---|---|
| `kind` | Shape of the change. See [Event kinds](#event-kinds) below. |
| `start_seconds`, `end_seconds`, `peak_seconds` | When, in source-video seconds. |
| `region_xyxy` | Bounding box of the change in **source** pixels, `[x0,y0,x1,y1]`. |
| `region_area_fraction` | That box as a fraction of the frame. |
| `position` | The third of the frame it sits in, e.g. `bottom right`. |
| `persists` | Whether the region still looks different afterwards. Absent when it could not be compared. |
| `continuous` | Set when activity runs steadily in one small fixed place for much of the interval, or when a `shift` is one step of movement already running there — the shape of animation rather than a fault. A claim about shape only: the tool cannot tell a marquee from a stuck render. `check` does not count continuous shifts against a layout-shift limit. |
| `moved_by_pixels` | Set on a `shift`: how far the content moved, in source pixels, positive Y down. Measured from the real frames either side, so it is exact regardless of `--analysis-width`. A move of more than about half its region leaves too little overlap to register and is reported as a change instead, so no `shift` is not evidence that nothing moved. |
| `layout_shift_score` | Share of the frame affected times how far it moved. CLS-shaped, not Chrome's CLS. Use it to rank and threshold. |
| `direction`, `travel_pixels` | Set when the active centre moves. |
| `jump_backwards_pixels`, `jump_backwards_seconds` | Set when the movement reverses once — a progress bar regressing, a scroll resetting, a carousel snapping back. The movement is usually expected; the jump is usually the bug. |
| `changes_per_second` | Set for `flicker`. Counts changes, so a full on-off cycle is two. |
| `peak_changed_fraction` | Largest share of the frame changing in one step. |
| `mean_changed_fraction` | Average share changing per step across the event. Far below the peak means one moment did most of it; close to it means a steady stretch. |
| `peak_drift_fraction` | Largest change across the `--drift` window. For `gradual` events this is the only non-zero measure. |
| `summary` | The same thing in a sentence. |

## Comparing two moments

`compare` returns `identical`, `changed_pixels`, `changed_fraction`,
`max_pixel_delta`, `mean_pixel_delta` and `differs_within_xyxy`.

| Result | Means |
|---|---|
| `identical: true` | Not one pixel differs. The frames are byte-for-byte the same picture. |
| `identical: false`, `changed_pixels: 0` | Nothing clears the threshold. On a lossy codec this is what "unchanged" actually looks like; `max_pixel_delta` tells you how far off it is. |
| `changed_pixels > 0` | Real difference. `differs_within_xyxy` bounds it in source coordinates. |

## Shift versus step

`step` and `shift` are both a one-off change that stays. The difference is what
happened to the content:

- **`step`** — something appeared, vanished, or changed. New pixels.
- **`shift`** — the same pixels, in a new place. It moved.

Only the second is a layout shift. Distinguishing them needs the actual frames,
so it happens in a second pass and is skipped when the recording is unsuitable.
A `step` where you expected a `shift` usually means the content changed as well
as moving, or the region was too small or too featureless to register against.

## A whole-frame change: new screen, or overlay?

A `cut` covers the whole frame, and two very different things look identical in
the numbers: the screen was replaced, or something translucent was put over it.
`uniform_shade_change` tells them apart by testing whether every pixel moved
through the same brightness map — which a re-shading of the same picture does,
and new content does not.

The map can **invert** as well as scale, and the sign of `shade_scale` is what
separates the two things that pass this test. Positive is something translucent
laid over the page. Negative is the page itself re-coloured: a dark-to-light
theme switch exchanges background and text, fitting a line of slope near −1.
Measured on a real toggle, 88% of the frame followed that map — the same share
a modal backdrop manages at +0.54.

| Field | Means |
|---|---|
| `uniform_shade_change: true` | The picture underneath is recoverable — only its brightness changed. Read `shade_scale` to see which kind. Absent or false does not mean "no overlay": a menu that opens without dimming the page behind it is new content, and reads as one. |
| `shade_scale` | The brightness multiplier, and its **sign is the finding**. About 0.5 means dimmed to half — a modal backdrop. Near −1 means inverted — the same content with light and dark exchanged, which is a theme switch. |
| `shade_fit` | The share of the frame that followed that map. A modal dims most of the frame and puts a dialog on the rest, so this says more than an average could: 0.85 means 85% dimmed and 15% is new content on top. |

Absent means it could not be judged: a blank frame before first paint can be
mapped onto anything, so the test is refused rather than guessed.

## What a result does not mean

- **An event is not a thing.** It is a region of pixels that changed together.
  One moving object can be several events; several objects moving together are
  one event.
- **A lossily compressed recording carries noise.** Measured on real screen
  captures: tens of pixels differ between any two frames of a static page, at
  up to 40/255. Scattered change is filtered out, but if a recording is heavily
  compressed and you are seeing events you cannot explain, raise `--threshold`
  to 20 or more and see which survive.
- **`region_xyxy` is a bounding box of change**, not an outline. A small object
  crossing the frame produces a box spanning its whole path.
- **Nothing found is not nothing happened.** Check `limits`. Re-run with
  `--threshold 4`, and with `--native` if the detail is thin.
- **A `stall` is an absence, not a thing.** It is reported when activity that
  had been running continuously stopped and then resumed in the same place, so
  it is meaningful on a recording with a spinner or a heartbeat and is
  deliberately never reported on a static screen, where a gap is just a gap.
  Its `region_xyxy` is the region that stopped, not a region that changed.
- **An `unsuitable` verdict means the event list is not a finding list.** On
  footage where most of the frame moves, events are fragments of one moving
  scene and their boundaries are where activity happened to cross a floor.
- **Colour in the `project` image is not source colour.** Read `encoding`, and
  read `omitted_from_image` — cuts, gradual events and stalls are not in the
  picture, and a glance at it alone can suggest nothing happened where plenty
  did.
- **A narrowed interval can make a page look unsettled.** `still_changing_at_end`
  means the recording ran out mid-change — and if you set `--end` yourself, you
  caused that. It describes the interval you asked for, not the recording.
- **Timestamps are decoder timestamps.** They are stable for one FFmpeg build
  and input, but seeking is keyframe-dependent, so treat them as accurate to
  roughly one frame rather than exact.

## When this tool is the wrong one

- Handheld or panning camera footage: everything changes, so everything is an
  event. Stabilise first, or sample frames directly with `frames`.
- Questions about *what a thing is* rather than *when it changed*: go straight
  to `sheet` and look.
- Audio: not analysed at all.

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

## Limits worth stating back

- No object recognition, no text reading, no explanation of cause.
- Timestamps are frame-scale. At 30fps every one is accurate to about 33ms, and
  seeking snaps to the nearest frame. Do not quote them more precisely, and
  expect a run at a lower `--sample-fps` to move them.
- Regions are bounding boxes of change, not object outlines.
- Analysis is downscaled to `--analysis-width` (320 by default) unless you pass
  `--native`; thin features can be missed.
- A move larger than about half the region it happened in cannot be registered
  and is reported as a change rather than a movement. No `shift` is not
  evidence that nothing moved.
- A moving camera, a scrolling page, a slow zoom, or ambient motion — wind in
  foliage, water, fire, a crowd, film grain — makes everything an event. The
  tool detects this itself and says so in `suitability`; believe it, and switch
  to `sheet` and `frames`.
- On a still screen, a gap is just a gap. A `stall` is only reported when
  something that *was* running continuously stopped, so a quiet stretch on an
  otherwise static recording is not one.
