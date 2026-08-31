# The images, and working without them

Four commands write PNGs: `sheet`, `project`, `frames` and `compare -o`. They
support the text answer; they are not the product. If you cannot open images,
skip to the last section — the text path is real, and stated honestly.

## `sheet` — what it looks like

```sh
agent-motion sheet recording.mp4
```

One PNG containing many real frames, each captioned with its timestamp and the
event it belongs to, choosing the moments from the analysis. Usually the
fastest way to learn what a recording is actually of. Pass `--at` for specific
moments, `--during` for one event's span, `--region` and `--pad` to crop every
tile the same way.

## `project` — where on screen the action was

```sh
agent-motion project recording.mp4
```

Returns everything `timeline` returns and additionally writes a PNG where every
pixel keeps its source `x,y`: red is how much it changed, green is when (black
early, bright late), blue is how often. Black is no change above the threshold.

It is an activity map, not a picture of the video, and it is not the whole
story. Whole-frame cuts are left out so they cannot flatten everything else,
`gradual` events barely register in it, and a `stall` cannot be drawn at all.
Everything it omits is named in `omitted_from_image` and printed into the
legend band. Read that before concluding nothing happened somewhere.

Use `sheet` when you want to know what something looks like, and `project` when
you want to know where on screen the action was.

## If you cannot look at images

Say so rather than guessing at their contents, and lean on the text instead.
Three commands answer spatial and comparative questions without a picture:

- `agent-motion activity` is `project` as text — one line per part of the frame
  that was busy while the rest held still, with a `box_xyxy` you can pass
  straight to `--region`. This is the direct substitute for the activity image.
- `agent-motion timeline` describes every event, with a region and a summary.
- `agent-motion compare` answers questions about two specific moments
  numerically — an exact changed-pixel count, the box those pixels fall in, and
  whether the two frames are identical.

That path is weaker, because nothing in it says *what* a region contains, but
it is a real one. Do not describe an image you have not seen.
