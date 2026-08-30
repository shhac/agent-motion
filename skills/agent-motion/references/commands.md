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
| `--at` | analysis chooses | Timestamps in seconds, e.g. `--at 3.4,7.1`. |
| `--during` | none | Sample evenly across a window, e.g. `--during 13.07:13.40`. Paste an event's `start_seconds` and `end_seconds`. Not combinable with `--at`. |
| `--count` | 12 | How many frames, for `--during` or when the analysis chooses. |
| `--columns` | auto | Grid columns. |
| `--width` | 320 | Thumbnail width. With `--region`, this magnifies the crop. |
| `--region` | none | Crop every tile to `x0,y0,x1,y1` in source pixels. Takes an event's `region_xyxy` verbatim. |
| `--pad` | 0 | Widen `--region` by this many pixels on every side. |
| `--quick` | off | Skip the analysis pass. Faster, but tiles lose their event labels. |
| `--output`, `-o` | `<video>.sheet.png` | Destination PNG. |

The sheet analyses the video even when you pass `--at`, so every tile is
labelled with the event it falls inside; `--quick` opts out. With `--at` the
result carries `narrative` and `suitability` but not the full analysis, which
you have already got if you ran `timeline`.

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
| `--at` | one of these | Timestamps in seconds. |
| `--during` | one of these | Sample evenly across a window, e.g. `--during 13.07:13.40`. |
| `--count` | 6 | How many frames `--during` takes. |
| `--dir` | `<video>.frames` | Destination directory. |
| `--width` | source width | Scale frames to this width. With `--region`, this magnifies the crop. |
| `--region` | none | Crop to `x0,y0,x1,y1` in source pixels. Takes an event's `region_xyxy` verbatim. |
| `--pad` | 0 | Widen `--region` by this many pixels on every side. |

`frames` writes one file per timestamp, so it takes `--dir` rather than
`--output`. For several moments side by side in a single image, use
`sheet --at ... --region ...` instead — it does the compositing for you.

Cropping happens before scaling, so `--region 200,120,202,160 --pad 24 --width
480` returns that region enlarged rather than a shrunken whole frame. This is
how you look at something too small to see in a full-frame still.

## `compare <video>`

Measures two arbitrary timestamps against each other.

| Flag | Default | Meaning |
|---|---:|---|
| `--at` | required | Exactly two timestamps, e.g. `--at 14.9,18.5`. |
| `--threshold` | 12 | Ignore per-pixel differences at or below this 0..255 value. |
| `--region` | none | Compare only `x0,y0,x1,y1` in source pixels. |
| `--pad` | 0 | Widen `--region` by this many pixels on every side. |
| `--output`, `-o` | none | Draw the difference to this PNG. |

Seeking snaps to the frame at or after the time you ask for, so two timestamps
a few milliseconds apart can land on the same side of a one-frame event and
report no difference. To straddle a moment, put one clearly before and one
clearly after; `compare` warns in `note` when they are too close to be sure.

`identical` means not one pixel differs. `changed_pixels` of zero with
`identical: false` means the difference is below the threshold — codec noise.
The drawn difference is at source resolution within the region, so use `--pad`
to give a thin feature room.

## `mcp`

Serves the same commands over MCP: `agent-motion mcp` on stdio, or
`--http <addr>`. A client that speaks MCP gets the same surface as the shell.

## `usage`

The compact agent-facing contract, same as this reference in short form.
