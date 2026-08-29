# agent-motion

Use `agent-motion` to create a deterministic temporal projection for a fixed-
viewport video when an image-capable agent needs to locate visual activity
without consuming a large atlas of frames.

## Core path

```sh
agent-motion project recording.mp4 --start 0 --end 30
```

The command writes a PNG and prints one JSON object. Read `output`,
`motion_coverage`, `peak_activity_time_seconds`, and especially `encoding`
before interpreting the image. v1 RGB does **not** represent source colour.

For a suspicious time range, recurse with a smaller interval:

```sh
agent-motion project recording.mp4 --start 17 --end 19
agent-motion project recording.mp4 --start 17.5 --end 17.9
```

Use original frames only after narrowing the interval enough to need them.

## Preconditions and limits

- Best for stationary-camera/viewport footage: UI captures, rendering, visual
  tests, browser behaviour, and game debugging.
- The first mode detects frame differences. It does not provide optical flow,
  camera stabilization, object identity, or video reconstruction.
- `--threshold` (default 12) suppresses small RGB deltas. Lower it for subtle
  jitter; raise it when compression noise overwhelms the map.
- `ffmpeg` and `ffprobe` must be installed. Pass `--ffmpeg` / `--ffprobe` to
  specify paths.

## Output and errors

Single result → JSON by default. `--format json|yaml|jsonl` overrides it.
Failures are JSON on stderr with `fixable_by`: `agent` means correct input,
`human` means a local dependency/output permission needs attention, and `retry`
means retry later.

Read [the project command reference](references/commands/project.md) for the
full flag and channel contract.
