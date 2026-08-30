# Behaviour reference

## Decoding

FFprobe reads dimensions, `avg_frame_rate`, duration, codec, pixel format and
audio presence from the first video stream. FFmpeg seeks to `--start`, decodes
for the requested duration, disables audio, subtitle and data streams, and
emits raw `rgb24` frames through `fps` and `scale` filters that pin the rate and
size the analyser asked for.

Results are deterministic for the same input, FFmpeg build, flags and pixel
format. FFmpeg's seeking is keyframe-dependent, so exact frame boundaries are
not promised across FFmpeg versions; treat timestamps as accurate to about one
frame.

## Two timescales

For each pixel between two frames:

```text
delta = (|dR| + |dG| + |dB|) / 3
```

A pixel counts as changed when `delta > threshold`.

**Fast**, against the previous frame. This finds anything abrupt.

**Slow**, against the frame `--drift` seconds earlier. This exists because fast
differencing is structurally blind to gradual change: a four-second fade moves
each channel by well under one unit per frame, so no per-frame threshold above
zero will ever see it. Over a one-second window the same fade is obvious.

The slow comparison uses two references, `drift` and `drift + 2` frames back,
and takes the smaller result. A single anomalous frame in the past would
otherwise reappear as a phantom slow change exactly one window later — a real
artefact, observed on a one-frame flash before this was added.

## Noise floor

Segmentation needs a floor, and a fixed one is wrong for both a lossless screen
capture and a heavily compressed camera clip. The floor is
`median + 6 × MAD` of the changed-fraction series, clamped to `[0.0004, 0.05]`.
It adapts to the recording rather than to an assumption about it.

## Segmentation is spatial as well as temporal

Statistics are kept per cell of an 8x6 grid, and segmentation runs per cell
before anything is merged. This is not an optimisation; it is what makes the
tool usable on real recordings. Almost every real capture has something
animating continuously — a spinner, a caret, a cursor, a clock — and a purely
temporal pass merges that with everything else and reports one event covering
the whole video. Measured on the defect scenario: a 20x20 pulsing dot produced
a single event spanning 376x144 px and hid both other faults.

Cells also improve sensitivity. A small change fills a much larger share of one
cell than of the whole frame, so a per-cell noise floor sees things a global
one cannot.

Cell segments are then merged into events when their cells touch (including
diagonally), their stretches overlap, **and** those stretches are within a
factor of eight in length. A moving object crossing several cells stays one
event; two things happening at once in different places stay two; and something
brief beside something long-running is not absorbed by it. Without the last
condition a progress bar advancing for twenty seconds swallows a caption that
moves for two frames.

## Segmentation steps

1. **Whole-frame transitions** — those changing at least half the frame — are
   pulled out first. A run of one is a `cut`; a run of one or two that does not
   persist is a `flash`; longer runs fall through to ordinary segmentation. They
   are removed before anything else so one enormous transition cannot swallow
   the events around it.
2. **Fast segments** are per-cell runs above that cell's noise floor, merged
   across gaps of up to 0.25s, which keeps a flicker from fragmenting into
   dozens of events.
3. **Slow segments** are per-cell runs of drift above the floor, excluding any
   time within one drift window after fast activity **in that same cell**. The
   mask has to be per cell: a constantly animating corner would otherwise hide
   a slow change happening anywhere else. They must last at least one window,
   and their start is shifted back by it, since drift reports a change one
   window after it began.

## Classification

Within a segment: `active` is the number of transitions that cleared the floor,
`duty` is that as a share of the span, `travel` is how far the activity centroid
moved relative to the typical per-frame footprint, and `persists` compares the
nearest retained checkpoint either side of the segment inside its region.

| Kind | Condition |
|---|---|
| `cut` | whole-frame, one transition, persists |
| `flash` | whole-frame, one or two transitions, reverts |
| `step` | at most 2 transitions, persists |
| `blip` | at most 2 transitions, reverts |
| `flicker` | 4+ transitions, duty below 0.75, centroid stationary |
| `motion` | centroid travels further than the typical footprint |
| `gradual` | found only in the slow pass |
| `busy` | anything else sustained |

`changes_per_second` counts changes, so a full on-off cycle is two. Direction is
the dominant axis of centroid travel, or both axes when neither dominates by 2:1.

A motion event also reports the largest step taken *against* its own direction,
when there is one larger than the typical per-frame footprint. Movement that
reverses once is a fault class the tool can see and would otherwise have no
words for — a progress bar regressing, a scroll resetting, a carousel snapping
back — where the movement is expected and only the discontinuity is the bug.

## Persistence

Up to 128 checkpoint frames at 96px wide are retained during the pass, thinning
by doubling their stride so an arbitrarily long stream stays bounded. A region
is "persistent" when its mean absolute difference between the checkpoints either
side exceeds `max(4, threshold/2)`. At 96px this is reliable for regions of a
few percent of the frame and unreliable below that, which is why `persists` is
omitted rather than guessed when there is nothing to compare.

## The activity image

Per pixel, over transitions that were not excluded as whole-frame:

- **Red** — accumulated magnitude, divided by the 99th percentile of non-zero
  magnitude, then square-rooted. The square root keeps low-amplitude but real UI
  motion visible beside one very bright region.
- **Green** — magnitude-weighted mean change time, mapped linearly from the
  first decoded frame to the last.
- **Blue** — `(changes + reversals) / transitions`, square-rooted.
- **Alpha** — always opaque. Inactive pixels are written as opaque black; left
  at the zero value they would be transparent, and a viewer would show its own
  background through them, inverting the whole reading.

Normalisation is local to one image, so values are not comparable between runs.
Gradual events contribute nothing, because they never clear the fast threshold.
Both facts are stated in the result rather than left to be discovered.

The optional legend band is appended *below* the frame, so image `x,y` still
maps to source `x,y`.

## Stalls

A hang or a freeze is an *absence* of change, so nothing in the pixel data
describes it. Reported only as a quiet range it reads exactly like a screen
meant to be still, which is the opposite of what it means.

A stall is therefore derived from the events rather than from the pixels. Two
events qualify as its ends when both run for at least 0.5s, their regions
overlap, the gap between them is at least 0.8s, and nothing else happened in
that region during the gap. Brief events are filtered out before the pairing,
or a blip landing inside a long event's span would break the adjacency between
that event and its own resumption.

The definition is deliberately narrow. On a static screen a gap is just a gap,
and calling that a freeze would make the finding worthless. It fires only when
something that *was* running continuously stopped and then started again.

The event carries the peak change seen during the gap, so a caller can tell
"nothing above the threshold" from "not a single pixel moved". The narrative
states it in its own sentence rather than leaving it in the quiet ranges.

## Comparing two moments

`compare` decodes exactly two frames and measures them against each other with
the same mean-absolute-RGB metric, at source resolution within the requested
region. It reports the changed count, the largest single-pixel difference and a
bounding box, and separates *identical* from *nothing above the threshold* —
on a lossy codec the second is what an unchanged screen actually looks like.

Seeking snaps to the frame at or after the time requested, so two timestamps
less than three frames apart can land on the same side of a brief event. The
result says so in `note` rather than leaving a reader to conclude nothing
changed.

The drawn difference is the later frame at 30% brightness with differing pixels
lit in proportion to the square root of their delta, so a one-shade change is
still visible beside a region that changed completely.

## Rotation

FFprobe reports coded dimensions; FFmpeg autorotates on output. For a source
with a quarter-turn rotation the frames that arrive are therefore the other way
up from the numbers in the probe, and because the scale filter forces the
requested size, nothing would have failed — every region and the whole
projection image would simply have been wrong. Width and height are swapped at
probe time when the rotation is not a multiple of 180.

## Grid sizing

Cells must be large enough that an event can exist in them. The per-cell floor
never drops below two pixels, so a cell of two pixels or fewer can never be
reported as active, and a small analysis width silently returned nothing at
all. The grid is coarsened until every cell holds at least twelve pixels.

## Suitability

Every analysis reports whether the recording is the kind the tool works on,
because on footage where everything moves the event list is a list of fragments
and nothing else in the output would say so.

The measure is the share of the analysed interval covered by an event spanning
more than 60% of the frame. A pan, a slow zoom, wind in foliage, water, fire, a
crowd and film grain all produce one; a dashboard with an animating widget does
not. Above 50% the verdict is `unsuitable`, above 20% `marginal`.

Two earlier measures failed and are worth recording. Counting grid cells above
their own noise floor scored panning footage as *more* suitable than a static
screen, because an adaptive floor rises to meet constant motion. The median
share of the frame changing per transition caught a pan but not foliage, where
only half a percent of pixels clear the threshold in a typical frame.
