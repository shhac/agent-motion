# Recipes

Task-shaped answers. Each assumes you have run `timeline` and have an event to
follow up on.

## Watching one event unfold

An event's start and end say a panel toggled ten times a second, or a colour
drifted for four seconds. Neither says what the toggle or the drift *looks*
like. Paste the event's own span back in and let the tool space the samples:

```sh
agent-motion sheet recording.mp4 --during 13.07:13.40 --count 10 \
  --region 498,38,582,102 --pad 12 --quick
```

`--during` works on `frames` too. `next_steps` proposes one of these for any
event with internal cadence.

## Seeing something small

A 20x20 indicator or a 2px layout shift is invisible in a full-frame still.
Crop to the region and magnify — `--region` takes an event's `region_xyxy`
verbatim, and cropping happens before scaling, so `--width` enlarges it:

```sh
agent-motion frames recording.mp4 --at 6.2 --region 200,120,202,160 \
  --pad 24 --width 480
```

`--pad` widens the crop so a thin feature is not flush against the edge. It
works on `sheet` too, which then crops every tile the same way — the fastest
way to watch one small element change over time.

## Narrowing in on something subtle

Events give you a range; run again inside it with a lower threshold to see what
was too small or too subtle the first time.

```sh
agent-motion timeline recording.mp4 --start 17 --end 19 --threshold 4
```

`--threshold` is the main dial. It is the per-pixel change, 0..255, that is
ignored. The default 12 suppresses compression noise and also hides genuinely
subtle rendering instability, so lowering it is the standard second move.

## Did the page finish loading?

Two fields, and the difference between them matters:

- `settled_at_seconds` — when *anything* last changed.
- `layout_settled_at_seconds` — when the *content* last changed, ignoring
  whatever merely keeps animating.

A ticker, a spinner or a video player keeps the first one late forever while the
page itself has been stable for seconds. For "has this finished loading", read
the second. Events with `continuous: true` are the ones doing that: activity
running steadily in one small fixed place for much of the interval. That is a
claim about shape, not meaning — the tool cannot tell a marquee from a stuck
render, and says so.

## Asking whether something is the same as it was

```sh
agent-motion compare recording.mp4 --at 14.9,18.5
agent-motion compare recording.mp4 --at 6.13,6.23 \
  --region 200,120,202,160 --pad 24 -o jitter.png
```

Every other command compares neighbouring frames. `compare` takes two arbitrary
timestamps and gives an exact pixel count, which answers questions nothing else
can: did the screen come back to the same state after that cut, did the region
really revert, is anything at all different between these two moments. It
distinguishes *identical* from *nothing above the threshold* — the second is
what codec noise looks like.

With `-o` it draws the difference: the later frame dimmed, with everything that
differs lit up. For a change of a pixel or two, this is the only way to see it —
two nearly identical stills cannot be compared by eye.

## Filtering a long event list

```sh
agent-motion timeline recording.mp4 --format jsonl | grep '"kind":"shift"'
```

`--format jsonl` renders the events one per line, with the narrative,
`suitability`, `limits` and the rest following as meta lines. Each event line
carries `kind`, `start_seconds`, `end_seconds`, `peak_seconds`, `region_xyxy`
and `position`, so filtering by kind, time or place needs no parsing of the
whole document. Read the `limits` meta line before concluding anything from
what you filtered out.
