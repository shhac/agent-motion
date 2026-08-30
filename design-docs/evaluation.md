# Evaluation

## Why a synthetic reference

Real footage cannot score a detector: you do not know what it should have
found. `internal/fixture` renders a scenario whose events are known to the frame
and the pixel, so accuracy is measurable and regressions are catchable.

```sh
make fixture   # writes .cache/eval/reference.mp4 and reference-truth.json
```

640x360, 30fps, 28 seconds, encoded with x264 at CRF 20 so that ordinary
compression noise is present rather than assumed away.

| Event | Time | Region | What it tests |
|---|---|---|---|
| `moving-dot` | 2.0–5.0s | 140,160–632,192 | travel, direction |
| `appear-badge` | 6.5s | 500,300–560,324 | one-off change that persists |
| `flicker-panel` | 9.0–12.0s | 300,60–380,140 | repetition, rate |
| `scene-cut` | 15.0s and 18.0s | whole frame | shot boundaries |
| `single-frame-flash` | 21.0s, one frame | whole frame | transient anomaly |
| `fade-region` | 23.0–27.0s | 200,200–400,320 | change too slow to see per frame |

Nothing else moves. Any additional event is a false positive.

`internal/motion/timeline_test.go` asserts each of these is found once, with the
right kind, within a second of the right time, over a region that overlaps the
true one by more than 60% without being more than 3x its area.

## The defect scenario

`make fixture SCENARIO=defect` renders a second, harder video: a dashboard that
is mostly still, with a dot in the header pulsing every 5 frames for the whole
20 seconds. Three faults hide behind that heartbeat.

| Event | Time | Region | What it tests |
|---|---|---|---|
| `layout-jitter` | 6.2s, 5 frames | 200,120–322,160 | a 2px shift beside a constant animation |
| `heartbeat-stall` | 11.0–14.0s | 556,16–576,36 | a freeze, which is an absence of change |
| `status-drift` | 16.0–19.0s | 450,250–530,290 | slow change beside a constant animation |

This scenario is what forced spatial segmentation. Before it, the heartbeat
produced one event spanning 376x144 px covering 10.7 seconds, and both the
jitter and the drift were invisible. After, the heartbeat is confined to its
own 20x20 region, the jitter is reported as the card's two moving edges, the
drift is found within a few pixels, and the stall is named in the narrative.

## Baseline

The first implementation, run over the whole reference video, reported five
numbers: 840 frames, 30fps, `motion_coverage` 1.0, `peak_activity_time` 15.0,
threshold 12. The image was a uniform green field. Nothing in that output
identifies a single one of the six events. That measurement is what motivated
decisions D1 through D4.

## Current

All six events are found with no false positives, and the events carry
direction, rate, region and persistence. See `decisions.md` for what each
correction cost.

## Agent trials

The tool is for agents, so it is tested by giving agents a video and no context
and asking what they can tell. Trials run in a scratch directory containing
only the binary, the skill and the media — no repository, no source, no ground
truth — so a trial cannot be contaminated by knowing the answer.

Findings and the changes they forced are recorded in
[`agent-trials.md`](agent-trials.md). Round 1 forced six changes; three of them
— stalls, region cropping, and the activity image naming its own omissions —
came from things an agent had to leave the tool to work out for itself.
