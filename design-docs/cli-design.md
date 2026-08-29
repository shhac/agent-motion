# CLI design

## Command surface (v1)

```text
agent-motion project <video> [--start seconds] [--end seconds]
                             [--threshold 0..255] [--output path]
                             [--ffmpeg path] [--ffprobe path]
agent-motion usage
```

The binary is named for the `agent-*` family. `project` is intentional rather
than `tshot`: it describes the transformation and leaves room for named modes
later. A thin `tshot` shell alias can be evaluated after the command contract
is stable.

`--end` is exclusive and optional. With no end, FFmpeg decodes to the source
end. The default output is `<input-without-extension>.temporal.png`.

## Output contract

`project` writes its PNG to disk and prints one JSON resource by default. It
contains at least:

```json
{
  "input": "recording.mp4",
  "output": "recording.temporal.png",
  "start_seconds": 12,
  "end_seconds": 18,
  "frames": 360,
  "fps": 60,
  "motion_coverage": 0.073,
  "peak_activity_time_seconds": 14.82,
  "threshold": 12,
  "encoding": { "red": "...", "green": "...", "blue": "..." }
}
```

The actual `encoding` strings are part of the API. Any transform change must
update them; an agent must never need to infer channel semantics from colour.

Future list operations default to NDJSON. `--format json|yaml|jsonl` is a
family-wide override; project defaults to JSON as a single resource.

## Failure contract

Every failure is one JSON line on stderr:

```json
{"error":"...","fixable_by":"agent|human|retry","hint":"..."}
```

- `agent`: invalid flags, unreadable/unsupported source.
- `human`: FFmpeg/FFprobe missing or the output location cannot be written.
- `retry`: process/I/O failure that may be transient.

No destructive source-video operation exists. Output is atomically written in
its target directory, so a failed encode does not leave a partial final PNG.
