# Decision log

Newest first. Each entry records what changed and what forced it.

## D9 — Segmentation is spatial as well as temporal

Statistics are kept per cell of an 8x6 grid and segmentation runs per cell
before merging. The defect scenario forced it: a 20x20 pulsing dot, the kind of
heartbeat almost every real capture has, produced one event spanning 376x144 px
across 10.7 seconds and hid both other faults entirely. Per-cell segmentation
confines the heartbeat to its own corner, and the two faults beside it — a 2px
card shift and a slow colour drift — are both found.

The drift mask had to become per cell for the same reason. Globally masked, the
constant heartbeat suppressed the slow-timescale pass everywhere.

A side effect worth having: a per-cell noise floor is more sensitive than a
global one, because a small change fills a much larger share of one cell than
of the whole frame.

## D8 — Family alignment

Added the conventions every sibling `agent-*` CLI has and this one lacked:
PolyForm Perimeter `LICENSE`, `.golangci.yml`, the shared release workflow, the
skill-publishing workflow, and YAML frontmatter on `SKILL.md`. The missing
frontmatter was the one with teeth: without `name` and `description` the skill
cannot be discovered at all.

## D7 — The fake decoder replays a known scenario

`video.Fake` honours the interval, rate and scale of a request and renders from
`internal/fixture`. Command-level tests therefore exercise real argument
handling, real segmentation and real image output with no FFmpeg and no media.
A fake that ignored the request would have made every test that uses it
meaningless, so its fidelity is itself tested.

## D6 — Results state their own limits and next move

`limits` and `next_steps` are in every analysis result. The failure mode this
prevents is a caller reading "no events" as "nothing happened" when the real
answer is "nothing above threshold 12 at 320px wide". The tool knows which
knob was too coarse; saying so costs a few tokens.

## D5 — The activity series is a sparkline by default

Sixty pretty-printed floats cost far more than they tell a reader. A sparkline
string conveys the shape of an interval in one line; `--series` returns the
numbers for a caller that wants them.

## D4 — Whole-frame transitions are excluded from the image

A single scene cut contributes enormous magnitude to every pixel at once, and
the p99 normalisation then flattens everything else to noise. Measured on the
reference video: whole-video `motion_coverage` was 1.0 and the image was a
uniform green field. Excluding transitions above 50% coverage brought coverage
to 0.10 and the structure back. The excluded timestamps are reported, and they
still appear as events.

## D3 — Two timescales, not one

Adjacent-frame differencing is structurally blind to gradual change: a
four-second fade moves each channel by well under one unit per frame, so no
threshold above zero can see it. The reference video's fade was invisible until
a second comparison against a delayed frame was added. This is a class of bug —
slow fades, easing animations, creeping layout shift — that the original design
could not have found at all.

The delayed comparison uses two references and takes the smaller, because a
one-frame flash otherwise reappears as a phantom slow change exactly one window
later. That artefact was observed before the fix.

## D2 — The primary output is text, not an image

The original design's thesis was that one spatially aligned PNG beats a frame
atlas. Building it and measuring it against a known scenario showed the
comparison was between the wrong two things: the cheapest and most accurate
answer to "what happens in this video" is a described timeline, and it costs a
fraction of either image.

The projection survives as one output among several. `timeline` became the
primary command; `project` is `timeline` plus the image. `sheet` was added
because knowing *what something looks like* still needs pixels, and a labelled
grid of real frames answers that better than an activity map does.

## D1 — Inactive pixels are opaque black

`image.RGBA`'s zero value is transparent, so every unchanged pixel in the
original projection had alpha 0. Any viewer with a light background rendered
the map inverted — a white field with coloured marks, described as "black means
no activity". Found by looking at the output rather than at the code. There is
now a test that walks every pixel and asserts full opacity.

## D0 — A synthetic reference video with known ground truth

`internal/fixture` renders a scenario containing exactly six things: a square
moving left to right, a badge appearing and staying, a panel flickering at a
known rate, two hard cuts, one all-white frame, and a slow fade. Ground truth is
known to the frame and the pixel, so detection can be scored rather than
eyeballed, and so unit tests can assert what the tool *should* find. Three of
the decisions above were found by running against it.
