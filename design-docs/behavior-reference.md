# Behaviour reference

## Decode

FFprobe reads dimensions and `avg_frame_rate` from the first video stream.
FFmpeg seeks to `--start`, decodes the requested duration when `--end` is set,
disables audio/subtitle/data streams, and emits raw `rgb24` frames. The result
is deterministic for the same input, installed decoder version, flags, and
pixel format; FFmpeg seeking itself can be keyframe-dependent, so exact frame
boundaries across different FFmpeg versions are not promised.

The source is never modified. The output is first written to a temporary file
in its destination directory and renamed only after PNG encoding succeeds.

## Difference and threshold

For each pixel p between adjacent frames:

```text
delta(p) = (abs(Rt-Rt-1) + abs(Gt-Gt-1) + abs(Bt-Bt-1)) / 3
```

Only `delta > threshold` is recorded. Therefore `--threshold 0` records every
non-identical RGB value; the default 12 deliberately suppresses small codec
and capture noise, but it can also hide subtle rendering instability. There is
no universal correct value.

## Per-pixel statistics

For above-threshold deltas the accumulator records magnitude, delta-weighted
time, count, and sign reversals. Sign is the direction of the RGB-sum luminance
change, and a reversal occurs when consecutive recorded signs differ. A
reversal is a heuristic hint of oscillation, not a guarantee of flicker.

`motion_coverage` is active pixels divided by all pixels. `peak_activity_time`
is the timestamp of the frame transition with the greatest number of active
pixels. It describes the selected interval in source-video seconds.

## Image normalisation

Red divides activity by the 99th percentile of non-zero accumulated magnitude,
then uses square-root contrast. Green maps the delta-weighted mean time from
the first to the last decoded frame. Blue maps `(change count + reversals) /
(frame count - 1)` with square-root contrast. These local mappings enhance
visibility but mean that RGB intensity is not a cross-run quantitative metric.
