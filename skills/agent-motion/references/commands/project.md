# `agent-motion project`

```sh
agent-motion project <video> [--start seconds] [--end seconds]
                             [--threshold 0..255] [--output path]
```

The command streams the selected interval through local FFmpeg, writes one PNG,
and returns a JSON resource. Default output is `<video>.temporal.png` (with the
existing extension removed). `--end` must be greater than `--start`; omitted
end means the source end.

| Flag | Default | Meaning |
|---|---:|---|
| `--start` | 0 | Inclusive approximate decode start, seconds. |
| `--end` | source end | Exclusive requested end, seconds. |
| `--threshold` | 12 | Ignore mean absolute RGB deltas at or below this number. |
| `--output`, `-o` | derived | Destination PNG. |
| `--ffmpeg` / `--ffprobe` | PATH names | Local decoder executables. |

The returned `encoding` map is authoritative. In v1 red is locally normalized
change magnitude, green is mean change time, and blue is frequency/reversal
activity. Black means no above-threshold activity. Use `peak_activity_time_seconds` as
a hint for the next narrower interval; it is not an exact bug timestamp.
