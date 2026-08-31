# Worked examples

Real output from real recordings, beside what was actually happening in them.
These are the four readings that are easiest to get backwards. Every number
here was measured, not invented.

## A modal opening vs a theme switching

Both change almost the whole frame at once, and both fit a brightness map over
88% of it. Both are therefore "not a new screen" — but one has something laid
over the page and the other is the page itself re-coloured. The sign of
`shade_scale` is what separates them.

A modal backdrop dimming a page:

```json
"kind": "busy", "start_seconds": 0.70, "peak_changed_fraction": 0.9646,
"uniform_shade_change": true, "shade_fit": 0.88, "shade_scale": 0.54
```

A dark-to-light theme toggle on the same site:

```json
"kind": "cut", "start_seconds": 5.10, "peak_changed_fraction": 0.9193,
"uniform_shade_change": true, "shade_fit": 0.88, "shade_scale": -0.93
```

Both are marked `uniform_shade_change: true`, which means the picture
underneath is recoverable — this was not a new screen. **The sign of
`shade_scale` is what separates them.**

Positive is something translucent laid over the page. 0.54 is the dim:
everything went to about half brightness, and the remaining 12% that did not
follow the map is the dialog that appeared on top.

Negative is the page itself re-coloured. −0.93 is an inversion: a theme flip
exchanges light and dark, so the background goes up while the text goes down,
and the best-fitting line has a slope near −1. Nothing was laid over anything —
there is no "remaining 12%" to explain, because nothing is on top.

The summaries say which: *"scaled to 54%, ... something translucent laid over
it"* against *"inverted, ... the same content with light and dark exchanged"*.

## A shift the tool can measure, and one it cannot

A real layout shift, on a news site:

```json
"kind": "shift", "start_seconds": 3.87, "moved_by_pixels": [-13, 0],
"region_xyxy": [468, 536, 548, 548]
```

"Content in the bottom centre moved left 13 px at 3.87s, and moves back."
Exact, in source pixels, measured from the two real frames either side.

The same tool on a page that jump-scrolled 659px in two frames:

```json
"kind": "busy", "start_seconds": 6.77, "end_seconds": 6.93,
"region_xyxy": [384, 60, 1280, 776], "region_area_fraction": 0.626
```

No `shift`, and that is the honest answer rather than a failure. A displacement
is found by registering one frame against the other, so it needs most of the
content still on screen afterwards. 659px out of a 740px-tall region leaves too
little overlap to register at all.

This case is the reason a displacement is verified against the pixels before
being reported. The profile correlation *did* produce an answer here — a
confident 198px, which is the spacing of a repeated card block on the page. A
page is deeply periodic, and a one-dimensional profile cannot tell a true match
from a periodic one. Undoing a real shift makes most of the changed pixels go
away; undoing that one did not, so it was rejected.

**A large region changing with nothing claiming to have moved is the signal to
look at the frames.** Two `sheet` tiles will settle in seconds what the
measurement declined to guess.

## An animation vs a layout settling

The foot of a personal site, animating throughout a 15-second recording:

```json
"kind": "shift", "start_seconds": 2.57, "moved_by_pixels": [0, 2],
"continuous": true
```

Eleven of these, every second or so, always in the same 124x64 box. Each one is
a real translation — the pixels genuinely moved two pixels down. None of them
is a layout bug.

`continuous: true` marks a shift that sits inside activity running steadily in
one fixed place for much of the interval. `check` does not count these against
`--max-shift-score` or `--max-shift-pixels`, and reports how many it excluded.
A gate that failed every page carrying a marquee is a gate nobody would leave
switched on.

This is a claim about **shape, not meaning**: the tool cannot tell a marquee
from a stuck render. It can only say that this has been moving in this spot the
whole time.

For "has the page finished loading", read `layout_settled_at_seconds` rather
than `settled_at_seconds` — the first ignores whatever merely keeps animating.

## An empty result that is not empty

`activity` over a quiet stretch between two modal transitions:

```
{"frame_wide": []}
{"grid": "8x6"}
{"noise_floor_fraction": 0.0015}
{"suitability": {"verdict": "suitable", ...}}
{"limits": ["Only activity local to a cell is listed. An empty list does not
  mean nothing happened: it means nothing happened in one place while the rest
  of the frame held still. Read frame_wide ...", ...]}
```

No cell records at all. The correct reading is "nothing happened in one place
here", not "nothing happened".

Over the whole recording instead, the same command returns eight cells covering
the centre of the frame — the dialog — and a `frame_wide` of four stretches:

```json
"frame_wide": [
  {"start_seconds": 0.73, "end_seconds": 0.90},
  {"start_seconds": 3.90, "end_seconds": 4.03},
  {"start_seconds": 5.37, "end_seconds": 5.53},
  {"start_seconds": 8.53, "end_seconds": 8.67}
]
```

Those four are the modal opening and closing twice. They are reported
separately because a whole-frame change lights all forty-eight cells equally
and so locates nothing; listing it per cell would bury the eight that actually
tell you where the dialog is.

**Cells answer "where", `frame_wide` answers "and when did everything move at
once".** Reading only one of them is how an empty list gets misreported.
