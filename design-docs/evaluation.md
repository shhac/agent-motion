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

## The layout scenario

`make fixture SCENARIO=layout` renders the content-shift test: an article page
where a banner and a late image each push everything below them down, plus a
badge that appears without moving anything.

| Event | Time | Region | What it tests |
|---|---|---|---|
| `banner-shift` | 2.0s | below y=60, down 40px | a large displacement |
| `inline-shift` | 4.0s | below y=250, down 24px | a second, smaller one |
| `badge-appear` | 6.0s | 700,20–760,44 | content appearing, which must **not** be called a shift |

The badge is the control and the reason the scenario exists. A tool that reports
every one-off change as a shift is no more useful than one that reports none.

Measured: both shifts are found with their displacement exact to the pixel
(40 and 24), and the badge is reported as a `step`.

## The player scenario

`make fixture SCENARIO=player` renders the generalisation test: a video-player
UI whose progress bar advances for the entire 22 seconds, with three faults
hiding behind it.

| Event | Time | Region | What it tests |
|---|---|---|---|
| `progress-regression` | 8.0s | 156,316–240,324 | a discontinuity inside expected movement |
| `thumbnail-flicker` | 13.0–13.4s | 500,40–580,100 | a short burst beside continuous motion |
| `caption-shift` | 17.0s | 60,240–400,276 | a 6px permanent shift beside continuous motion |

It exists because two scenarios can be tuned against; a third that the rules
have never seen tests whether they generalise. It immediately found that they
did not: the progress bar absorbed the caption and the regression, reporting
one event covering the bottom half of the frame. See D18.

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

## Real recordings

Every scenario above is synthetic, which makes them scoreable and makes them
unrepresentative. [`capture-real-recordings.md`](capture-real-recordings.md)
instructs an agent with a browser and a screen recorder to capture real page
loads — chosen for content shift, with stable controls alongside — into
`.cache/eval/real/`. It is written to be handed to a tool that does not have
`agent-motion` at all.

## What real recordings changed

Eight real page loads were captured headlessly — Forbes, CNN, AllRecipes, the
Guardian, the Independent, ESPN, plus Wikipedia and Hacker News as stable
controls — at 1280x800 with a cold cache, and transcoded to constant frame
rate. Three things came out of them that no synthetic fixture could have shown.

**The false positives were all compression noise.** A static Wikipedia article
produced seven events. Measured with `compare`, the worst was 31 pixels of
1,024,000 differing, scattered across a 949x557 box — lossy encoding, not the
page. Real recordings are lossily compressed and synthetic ones rendered from
clean sources, so the fixtures could never have surfaced this.

The fix is density: something that genuinely changed is solid within its own
bounds, while codec noise is a scatter across a large box. As a ratio of two
frame fractions it holds at any resolution, where a pixel count would not. It
took Wikipedia from seven events to two and left every fixture untouched.

Two things were tried and rejected on the evidence. Raising the per-cell pixel
floor made real pages *worse*, fragmenting runs into more and shorter events,
and broke a fixture at eight pixels. Extending shift detection to whole-frame
changes found nothing at all.

**A real page re-layout is not a translation.** The dominant change on a real
load — the moment the article re-flows — registers as a whole-frame `cut`, and
correlating it finds no displacement, because content re-wraps and resizes
rather than sliding. The layout fixture, solid blocks translating exactly, is
not representative of that case. Shift measurement works, and is demonstrated
to the pixel on the fixtures and on the player scenario, but on these eight
pages it fired only on trivia. That is a real limit, not a tuning problem.

**No shifts were invented.** Across eight real recordings the tool reported no
false layout shift, and the two controls reported none at all. Conservative is
the right direction to fail in.

The captures also demonstrated the trap the capture instructions warn about:
Playwright records variable frame rate, and it had to be transcoded before any
of this was trustworthy.

### A trial on a real recording

The fourth round put an agent on a real Forbes page load rather than a fixture.
It answered the ticket correctly and rated the tool 7/10, and its two complaints
became D26: a modal backdrop that was indistinguishable from a theme change, and
a settle time that a scrolling ticker rendered meaningless. Its verdict on the
page — no genuine layout-shift bug, and `check` passing — held up.

## A real browser doing a real reflow

The synthetic layout fixture translates solid blocks, which left an open
question the eight real page loads could not settle: does shift measurement fire
on an actual layout engine at all, or only on rendered rectangles?

A local page answers it with ground truth intact. A 600x200 image with no
`width` or `height` — the textbook cause of layout shift — is served two seconds
late, so the browser lays the page out without it and reflows when it lands,
pushing everything below down by exactly 200px. Real Chrome, real reflow, real
lossy recording, known answer.

It measures **exactly 200px**. That is the strongest evidence in this document.

Getting there exposed two false positives that the synthetic fixtures could not
have produced, because both need a real page's whitespace:

- The same frames also yielded a confident **426px sideways move that never
  happened**. A page is mostly white, so a column profile can be slid a long way
  and still look better than not sliding it.
- **First paint was reported as content moving.** A blank page has a profile
  spread of exactly zero, so every offset fits it equally.

Both are now refused, on measurements rather than intuition: the true vertical
offset leaves a residual of 0.05 of the profile's own spread, the spurious
horizontal one leaves 3.3, and a blank page leaves a spread of 0.000.

## Eight real page recordings

Captured externally to the instructions in `capture-real-recordings.md`: ffmpeg
AVFoundation screen capture of a real browser, constant 30fps, CRF 18, cropped
to a 1280x800 viewport, fifteen seconds each. Six ad-supported news pages and
two stable controls.

| Result | Evidence |
|---|---|
| No false positives on the controls | Wikipedia and Hacker News report **zero events**; `compare` confirms zero pixels above threshold across the full fifteen seconds, max single-pixel delta 7/255 |
| The overlay test holds on footage it was not tuned against | Forbes' consent backdrop is reported as a uniform brightness change scaled to 44%; the contact sheet shows the page visibly dimming |
| A marquee looked like seven layout shifts | Forbes' TRENDING ticker slides 2px at a time, and each step is a real translation. Now marked `continuous` and excluded from the gate — see D29 |
| An empty result dropped its own contract | `events` vanished when empty, crashing the script reading it. Fixed, with a test |

They are warm reloads rather than cold navigations, and the sidecar `observed`
notes describe the end state rather than what moved, so their ground truth is
weaker than the fixtures'. Everything concluded from them is checkable in the
pixels.

## Agent trials

The tool is for agents, so it is tested by giving agents a video and no context
and asking what they can tell. Trials run in a scratch directory containing
only the binary, the skill and the media — no repository, no source, no ground
truth — so a trial cannot be contaminated by knowing the answer.

Findings and the changes they forced are recorded in
[`agent-trials.md`](agent-trials.md). Round 1 forced six changes; three of them
— stalls, region cropping, and the activity image naming its own omissions —
came from things an agent had to leave the tool to work out for itself.
