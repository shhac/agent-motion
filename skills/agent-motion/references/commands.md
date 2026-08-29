# `agent-motion` commands

Every command takes one video path. Global flags: `--format json|yaml|jsonl`,
`--ffmpeg`, `--ffprobe`, `--timeout`, `--color`, `--debug`.

## Shared analysis flags

Used by `timeline`, `project`, and by `sheet` when it chooses its own moments.

| Flag | Default | Meaning |
|---|---:|---|
| `--start` | 0 | Interval start, seconds. |
| `--end` | end of video | Interval end, seconds. Must exceed `--start`. |
| `--threshold` | 12 | Per-pixel change, 0..255, to ignore. The main sensitivity dial. |
| `--drift` | 1 | Also compare each frame against the frame this many seconds earlier, which is what makes slow change visible. `0` disables it. |
| `--analysis-width` | 320 | Downscale to this width before analysing. |
| `--native` | off | Analyse at source resolution instead. `project` always does. |
| `--sample-fps` | every frame | Analyse this many frames per second. |
| `--max-events` | 40 | Cap on reported events; the rest are counted in `events_omitted`. |
| `--buckets` | 60 | Resolution of the activity sparkline. `0` omits it. |
| `--series` | off | Also emit the numeric activity buckets. |

## `inspect <video>`

Container and stream facts only — no decoding. Safe on a very large file.

```sh
agent-motion inspect recording.mp4
```

## `timeline <video>`

The main command. Decodes the interval once and describes it.

```sh
agent-motion timeline recording.mp4 --start 12 --end 18 --threshold 6
```

## `sheet <video>`

Writes one PNG of many labelled frames.

| Flag | Default | Meaning |
|---|---:|---|
| `--at` | analysis chooses | Timestamps in seconds, e.g. `--at 3.4,7.1`. Skips analysis. |
| `--count` | 12 | How many frames when the analysis chooses. |
| `--columns` | auto | Grid columns. |
| `--width` | 320 | Thumbnail width. |
| `--output`, `-o` | `<video>.sheet.png` | Destination PNG. |

## `project <video>`

Everything `timeline` returns, plus the activity-map PNG. Always analyses at
source resolution, so it is the slowest command.

| Flag | Default | Meaning |
|---|---:|---|
| `--output`, `-o` | `<video>.temporal.png` | Destination PNG. |
| `--legend` | on | Append a legend band below the frame. It adds rows underneath and never moves a frame pixel, so `x,y` still maps to the source. |

## `frames <video>`

Writes real source frames.

| Flag | Default | Meaning |
|---|---:|---|
| `--at` | required | Timestamps in seconds. |
| `--dir` | `<video>.frames` | Destination directory. |
| `--width` | source width | Scale frames to this width. |

## `usage`

The compact agent-facing contract, same as this reference in short form.
