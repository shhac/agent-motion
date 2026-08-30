# Decision log

Newest first. Each entry records what changed and what forced it.

## D19 — A backwards step inside movement is named

Movement that reverses once is a class of fault the tool could see and had no
words for: a progress bar regressing, a scroll position resetting, a carousel
snapping back. The movement itself is expected, so reporting only "something
moved left to right" describes everything except the bug.

Motion events now carry the largest step taken against their own direction of
travel, and the summary says so. The threshold is the typical per-frame
footprint, so the jitter of a bounding box around a moving thing does not
qualify. Verified to fire on a deliberate snap-back and to stay silent on the
reference scenario's smooth traverse.

## D18 — Cells merge on comparable duration, not just adjacency

A third scenario, built to test generalisation rather than to tune against,
broke the segmentation immediately. A progress bar advancing for the whole
recording absorbed everything adjacent to it: a caption dropping six pixels and
a backwards jump in the bar both vanished into one event covering the bottom
half of the frame.

Adjacency alone was the wrong rule. Two cells belong to the same event when
they are active *together*, not when one is active while the other happens to
be running, so a merge now also requires the two stretches to be within a
factor of eight in length. All three faults are found, and neither of the two
earlier scenarios changed.

This is the same failure as D9 one level up: that one was about two things in
different places, this one about two things on different timescales.

## D17 — Rotation is applied, not just reported

A quarter-turn source would have produced entirely wrong coordinates without
failing: FFprobe reports coded dimensions, FFmpeg autorotates on output, and
the scale filter forces the requested size, so the frames simply arrived the
other way up and every region described the wrong part of the picture.
Dimensions are now swapped at probe time.

## D16 — Zero is a value, not a missing value

`--threshold 0` was validated as legal, documented as the most sensitive
setting, and then silently replaced with the default of 12. `--buckets 0` was
documented in two places as omitting the activity series and silently gave 60.
Both came from `withDefaults` treating zero as "unset".

Options now take zero literally, a negative value means "use the default", and
`engine.Defaults(path)` is how callers get the documented settings. The
sentinel round trip between the CLI and the engine for "drift off" went away
with it.

A related gap: `--sample-fps` had no coverage and no mention in `limits`, so
sampling the reference video at 1 fps reported its 10 Hz flicker as three
one-off changes — confidently wrong, with nothing to warn a reader. Both the
sampled rate and a disabled drift window now appear in `limits`.

## D15 — Compare answers "is it the same as it was"

Three of six evaluation agents, independently and across both rounds, named the
same missing capability: a way to measure two arbitrary timestamps against each
other. Every other command compares neighbouring frames, so "did the screen come
back after that cut", "did the region really revert" and "is anything at all
different here" could only be answered by putting two stills side by side and
squinting — one agent reached for ImageMagick, another said it would have turned
an inference into a fact.

`compare` returns an exact pixel count and separates *identical* from *nothing
above the threshold*, which on a lossy codec is the distinction that matters.
With `-o` it draws the difference, because two nearly identical stills cannot
be compared by eye — which is exactly when the answer matters most.

## D14 — The activity image names what it leaves out

An agent found that `project` silently dropped four of seven events — every
cut, the flash and the gradual fade — leaving an image that suggested nothing
happened in the second half of the clip. The exclusions were disclosed, but
only in a JSON field, and the image is the thing people look at first.

The legend band now carries the omission line, in the picture, and the result
carries the same sentence as `omitted_from_image`.

## D13 — Regions can be cropped and magnified

Both debugging trials left the tool for ImageMagick, because a 20x20 indicator
or a 2px card edge is invisible in a 640px still. For a tool whose findings are
routinely a few pixels across, that is a hole in the middle of the workflow.

`--region` and `--pad` on `frames` and `sheet` crop before scaling, so `--width`
magnifies. `--region` takes an event's `region_xyxy` verbatim, and `next_steps`
now proposes a crop for the smallest event found.

## D12 — A stall is a finding, not a quiet range

The most diagnostically important fact in the defect scenario — a 3.2 second
freeze — reached the caller as `quiet_ranges: [[10.83, 14]]` and the sentence
"Nothing changes during 10.83s-14.00s". An agent debugging a "felt janky"
report had to re-run at a lower threshold and diff frames with an external tool
to establish what the tool already knew. Its verdict: the vocabulary "actively
undersells it — quiet reads as fine, not the screen froze".

A freeze is an absence of change, so no pixel describes it; it has to be
derived. `stall` is now an event kind, reported when activity that was running
continuously stopped and resumed in the same place, and the narrative says so
in its own sentence. The definition is narrow on purpose: on a static screen a
gap is just a gap, and a stall reported there would be worthless.

## D11 — The sparkline shows fast change only

It previously took the larger of the fast and slow measures, which saturated on
any continuous texture — shimmering foliage, film grain — and a one-line
summary that is always full is not a summary. Gradual events are reported in
the event list instead.

## D10 — Results assess their own suitability

Given ten seconds of animated film, the tool returned fourteen confident event
summaries, two of them describing "repeated toggling across the whole frame".
Nothing in the output said the findings were fragments of one continuously
moving scene.

Every analysis now reports a `suitability` verdict from the median share of the
frame changing per transition, and the narrative leads with the warning when
the verdict is not `suitable`. Any event covering more than 60% of the frame is
relabelled and described as whole-frame motion rather than as a localised
finding.

The measure had to be absolute. The first attempt counted grid cells above
their own noise floor, which scored panning footage as *more* suitable than a
static screen: the adaptive floor normalises away exactly what the question is
asking about.

Calibration also corrected a wrong assumption. The animated clip used for this
turned out to be a near-static shot, so its "suitable" verdict was right all
along; a deliberately panned version was needed to calibrate the threshold.

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
