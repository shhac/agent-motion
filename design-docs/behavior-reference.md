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

## Segmentation

1. **Whole-frame transitions** — those changing at least half the frame — are
   pulled out first. A run of one is a `cut`; a run of one or two that does not
   persist is a `flash`; longer runs fall through to ordinary segmentation. They
   are removed before anything else so one enormous transition cannot swallow
   the events around it.
2. **Fast segments** are runs above the noise floor, merged across gaps of up to
   `--merge-gap` (0.25s), which keeps a flicker from fragmenting into dozens of
   events.
3. **Slow segments** are runs of drift above the floor, excluding any time
   within one drift window after real fast activity, and must last at least one
   window. Their start is shifted back by the window, since drift reports a
   change one window after it began.

## Classification

Within a segment: `duty` is the share of frames that were active, `travel` is
how far the activity centroid moved relative to the typical per-frame footprint,
and `persists` compares the nearest retained checkpoint either side of the
segment inside its region.

| Kind | Condition |
|---|---|
| `cut` | whole-frame, one transition, persists |
| `flash` | whole-frame, one or two transitions, reverts |
| `step` | at most ~2 frames long, persists |
| `blip` | at most ~2 frames long, reverts |
| `flicker` | 4+ active transitions, duty below 0.75, centroid stationary |
| `motion` | centroid travels further than the typical footprint |
| `gradual` | found only in the slow pass |
| `busy` | anything else sustained |

`changes_per_second` counts changes, so a full on-off cycle is two. Direction is
the dominant axis of centroid travel, or both axes when neither dominates by 2:1.

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
