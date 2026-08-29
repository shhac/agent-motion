# Reading `agent-motion` results

## Result fields

| Field | What it is |
|---|---|
| `narrative` | One paragraph covering the whole interval. Read it first. |
| `events` | The described occurrences, in time order. |
| `quiet_ranges` | Stretches with no detected change. Two ranges meeting at one timestamp are separated by an instantaneous event, not joined. |
| `busiest_seconds` | Timestamp of the single largest frame-to-frame change. |
| `activity_sparkline` | Shape of the interval, one character per bucket, `_` to `#`. |
| `activity_sparkline_full_scale` | The value a `#` represents. The ramp is square-root scaled, so it is orientation, not measurement. |
| `motion_coverage` | Fraction of pixels that changed at least once. |
| `timestamps_worth_inspecting` | Frames that would show the events found. |
| `next_steps` | Commands you can run verbatim. |
| `limits` | What this run could not have seen. |
| `analysis` | Exactly how the pass was done, so it can be reproduced. |

## Event fields

| Field | What it is |
|---|---|
| `kind` | Shape of the change. See the table in SKILL.md. |
| `start_seconds`, `end_seconds`, `peak_seconds` | When, in source-video seconds. |
| `region_xyxy` | Bounding box of the change in **source** pixels, `[x0,y0,x1,y1]`. |
| `region_area_fraction` | That box as a fraction of the frame. |
| `position` | The third of the frame it sits in, e.g. `bottom right`. |
| `persists` | Whether the region still looks different afterwards. Absent when it could not be compared. |
| `direction`, `travel_pixels` | Set when the active centre moves. |
| `changes_per_second` | Set for `flicker`. Counts changes, so a full on-off cycle is two. |
| `peak_changed_fraction` | Largest share of the frame changing in one step. |
| `peak_drift_fraction` | Largest change across the `--drift` window. For `gradual` events this is the only non-zero measure. |
| `summary` | The same thing in a sentence. |

## What a result does not mean

- **An event is not a thing.** It is a region of pixels that changed together.
  One moving object can be several events; several objects moving together are
  one event.
- **`region_xyxy` is a bounding box of change**, not an outline. A small object
  crossing the frame produces a box spanning its whole path.
- **Nothing found is not nothing happened.** Check `limits`. Re-run with
  `--threshold 4`, and with `--native` if the detail is thin.
- **Colour in the `project` image is not source colour.** Read `encoding`.
- **Timestamps are decoder timestamps.** They are stable for one FFmpeg build
  and input, but seeking is keyframe-dependent, so treat them as accurate to
  roughly one frame rather than exact.

## When this tool is the wrong one

- Handheld or panning camera footage: everything changes, so everything is an
  event. Stabilise first, or sample frames directly with `frames`.
- Questions about *what a thing is* rather than *when it changed*: go straight
  to `sheet` and look.
- Audio: not analysed at all.
