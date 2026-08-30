# Capture run notes — 2026-08-30

## Status

No recordings were created in this run. The intended output directory exists:

```text
.cache/eval/real/
```

but contains no capture artifacts from this attempt.

## Why this run stopped

The available remote browser could navigate pages but could not produce viewport
screenshots. The connected desktop-browser surfaces were unavailable. That
makes it impossible to make a valid viewport-only, constant-frame-rate video
from this session. Producing an MP4 anyway would risk exactly the invalid
inputs `capture-real-recordings.md` rules out: an incomplete capture, a
variable/unknown sampling rate, or UI chrome rather than the page viewport.

## Local follow-up

Run the capture procedure in `capture-real-recordings.md` from a browser with a
fresh/cold profile and a recorder that can capture the viewport at 1280×800,
30fps CFR. For each completed capture:

1. Verify `r_frame_rate` and `avg_frame_rate` with the documented `ffprobe`
   command; transcode to CFR when they differ materially.
2. Save `<site>-<scenario>.mp4` and its same-name Markdown sidecar in
   `.cache/eval/real/`.
3. Record only observations actually seen, including uncertainty and approximate
   timestamps/locations.

Start with the two controls (`wikipedia-load`, `hackernews-load`) to validate
the recorder, then capture the likely-shift pages before the two deliberate
edge cases (`bbc-nav`, `guardian-scroll`).
