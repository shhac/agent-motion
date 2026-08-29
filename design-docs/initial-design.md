# Temporal Image Projection for LLM Video Debugging

## Thesis

`agent-motion` turns a selected video interval into one ordinary, spatially
aligned PNG plus deterministic JSON metadata. It is an observation primitive
for a multimodal agent, not a video-reconstruction or action-recognition
system.

```text
video (x, y, t) → temporal projection → PNG (x, y, channels) + metadata
```

The useful first environment is fixed-viewport video: UI recordings, browser
behaviour, visual tests, rendering, animation, and games. In that setting a
pixel's coordinates are meaningful across frames, while conventional frame
sampling repeatedly spends visual tokens on the static screen.

## Intended observations

- movement and approximate path/direction
- flicker and repeated oscillation
- one-time appearance/disappearance
- CSS/layout jitter and subpixel instability
- fades, timing anomalies, and unexpected redraws

Absolute frame differences alone establish activity, not direction or cause.
The projection therefore remains an investigative cue: the agent should use it
to select a smaller interval, then request frames if needed.

## Temporal zoom workflow

```text
whole recording
  → projection identifies suspicious range
  → smaller projection
  → still smaller projection
  → individual frames only when necessary
```

For example: 0–30 seconds → 17–19 seconds → 17.5–17.9 seconds. This is the
time-dimensional analogue of cropping an image before inspecting it closely.

## Existing techniques and position

The component techniques are established: frame differencing, motion history
and motion energy images, dynamic images/rank pooling, optical flow, and
video-model temporal token compression. The product hypothesis is different:
a deterministic, inspectable, cacheable, model-independent PNG designed for
tool calls by any image-capable LLM, paired with agent-controlled temporal
zoom.

## v1 decision

Implement a frame-difference activity map before optical flow or ML:

1. Decode the requested interval as `rgb24` with local FFmpeg.
2. Compute mean absolute RGB difference from each previous frame.
3. Suppress deltas at or below a configurable threshold.
4. Accumulate per-pixel magnitude, weighted time, change count, and luminance
   sign reversals.
5. Write one PNG and explain every field/channel in JSON.

The initial colour channels are deliberately diagnostic rather than pretty:

| Channel | v1 value | Signal |
|---|---|---|
| R | p99-normalized accumulated delta | total activity / trail strength |
| G | mean normalized activity time | early-to-late chronology |
| B | change frequency plus reversals | flicker/oscillation versus one change |

Black means no detected above-threshold change. Because normalisation is local
to a projection, RGB values are not comparable across independently generated
images; metadata makes that explicit.

## Non-goals for v1

- video reconstruction or visually natural output
- reliable object identity or semantic motion interpretation
- camera-motion handling / registration
- optical flow
- a model-specific video-token pipeline
- a claim of novelty in the underlying CV operations

## Core experiment

Test the hypothesis, not the aesthetics. Produce short recordings with known
UI/rendering bugs and give an image-capable model the same task under:

1. sampled-frame atlas
2. one temporal projection
3. temporal projection plus recursive temporal zoom

Measure bug detection, timestamp localization, explanation quality, false
positives, visual input cost, and tool calls. The central hypothesis is that,
for temporally sparse visual debugging, spatially aligned projection enables
more accurate and/or efficient reasoning than conventional frame sampling.
