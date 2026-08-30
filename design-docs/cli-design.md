# CLI design

## Command surface

```text
agent-motion inspect  <video>
agent-motion timeline <video> [analysis flags]
agent-motion sheet    <video> [--at ...] [--count N] [--columns N] [--width N]
                              [--region x0,y0,x1,y1] [--pad N] [--quick] [-o path]
agent-motion project  <video> [analysis flags] [-o path] [--legend]
agent-motion frames   <video> --at ... [--dir path] [--width N]
                              [--region x0,y0,x1,y1] [--pad N]
agent-motion usage
```

Analysis flags shared by `timeline`, `project` and a self-choosing `sheet`:
`--start --end --threshold --drift --analysis-width --native --sample-fps
--max-events --buckets --series`.

## Why this shape

The commands are ordered by cost, and each one's output points at the next.

`inspect` decodes nothing. `timeline` decodes once and answers in text. `sheet`
answers in one image. `project` is `timeline` plus an activity map, at source
resolution, so it is the most expensive. `frames` is the last step, once a
timestamp is known.

`project` returning the whole timeline is deliberate: an agent that wants the
picture also wants the story, and the analysis pass that draws the picture has
already produced it. Charging a second command for it would be a worse deal.

## Output contract

One JSON object on stdout per invocation; `--format json|yaml|jsonl` overrides
the default. Keys are sorted, matching the family convention.

The fields an agent is expected to read, in order: `narrative`, `events`,
`limits`, `next_steps`. Everything else is detail.

Four fields are load-bearing and must not be quietly dropped:

- **`suitability`** says whether the events mean anything at all. Without it,
  footage where everything moves returns a list of confident findings that are
  fragments of one moving scene.

- **`limits`** states what the run could not have seen. Without it, "no events"
  reads as "nothing happened", which is the most likely way this tool misleads.
- **`next_steps`** gives runnable commands. It is what makes the temporal zoom
  loop something an agent falls into rather than has to invent.
- **`encoding`**, on `project`, defines the image. It is the API for the PNG;
  an agent must never infer channel meaning from colour. `omitted_from_image`
  is part of the same contract and is drawn into the legend band as well, so a
  reader who looks at the picture before the JSON still sees it.

The activity series is a sparkline string by default. Sixty pretty-printed
floats cost far more than they tell you; the numbers are available behind
`--series` for a caller that wants them.

## Cropping

`--region` takes the four numbers an event already reports in `region_xyxy`, so
a region can be pasted straight from a result with no arithmetic. Cropping
happens before scaling, so `--width` magnifies the region rather than shrinking
the frame around it — which is the entire point, since the features this tool
finds are routinely a few pixels across.

`sheet` analyses even when given `--at`, so tiles carry event labels; with
`--at` it returns `narrative` and `suitability` but not the full analysis,
which the caller has already got if they ran `timeline`.

## Naming

`timeline`, `sheet`, `frames` and `inspect` say what you get. `project` is kept
from the original design and now means the activity-map projection specifically,
which is one output rather than the product.

Event kinds — `cut`, `flash`, `step`, `blip`, `flicker`, `motion`, `gradual`,
`busy` — name the *shape* of a change. They deliberately avoid words like
"appeared" or "glitch" that would assert meaning the tool cannot establish.

## Failure contract

Every failure is one JSON object on stderr with a non-zero exit:

```json
{"error":"...","fixable_by":"agent|human|retry","hint":"..."}
```

- `agent` — bad flags, a timestamp outside the video, an unreadable source.
- `human` — FFmpeg or FFprobe missing, or the output location cannot be written.
- `retry` — a transient decoder or I/O failure.

Nothing modifies the source video. Images are written to a temporary file in
the destination directory and renamed only after encoding succeeds, so a failed
run never leaves a partial PNG in place of a good one.
