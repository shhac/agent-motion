# Decision log

Newest first. Each entry records what changed and what forced it.

## D21 — The family conventions that were actually missing

A survey of nine sibling `agent-*` CLIs found four things this repo lacked
beyond the licence and lint config: the `mcp` command that every mature sibling
registers, `.claude/commands/release.md`, a repo-level guard on the skill's
frontmatter, and `make tidy`. All four are now here.

Two deliberate deviations remain, both recorded rather than drifted into:

The family's idiom for a subprocess seam is a `runCmd func(...)` field on a
struct. This repo uses a `video.Decoder` interface with an FFmpeg
implementation and a `Fake` that replays a scenario. The interface earns its
keep here because the fake is not a stub — it renders real frames, honours the
interval, rate and crop of a request, and is what lets command-level tests
exercise segmentation and image output with no decoder installed.

The family is beginning to return file outputs as `output.FileRef` rather than
as a bare path. Two of nine repos do so far. `project`, `sheet` and `frames`
return plain paths, which three rounds of agent trials read without difficulty.
Worth revisiting when the convention settles; not worth changing a contract
that has been evaluated.

## D24 — Density, not size, separates change from compression noise

A static Wikipedia article, recorded and re-encoded, produced seven events. The
worst was 31 pixels of 1,024,000 differing, scattered across a 949x557 box.
Synthetic fixtures are rendered from clean sources and could never have shown
this; it took real recordings.

A brief event is now rejected when its changed pixels fill less than 1% of the
region they span. Something that genuinely changed is solid within its bounds —
a 2px card edge sliding measures 0.03, a badge appearing 1.0 — while codec noise
measures 0.0002. Two orders of magnitude apart, and expressed as a ratio of two
frame fractions it holds at any resolution, where a pixel count would not.

Only brief events are judged this way. Movement and sustained activity are
legitimately diffuse: a small object crossing the frame covers a large region
while changing little of it at any instant.

Two alternatives were tried and rejected on the evidence. Raising the per-cell
pixel floor made real pages worse — it fragments runs into more, shorter events
rather than removing them — and broke a fixture. Applying the density test per
cell did nothing, because a couple of adjacent noise pixels look perfectly dense
inside one cell; the scatter is only visible across the whole region.

## D23 — Check turns the analysis into a pass or fail

Finding a regression is only half of catching one. `check` asserts conditions
and exits non-zero, so a visual regression can break a build rather than
waiting to be noticed by someone reading JSON.

Two decisions inside it. Every threshold is opt-in and an unset flag asserts
nothing — otherwise a zero default silently becomes the strictest possible
check, and every run fails for reasons nobody chose. And every failure carries
the event that broke it, because a CI log saying only "failed" leaves whoever
reads it exactly where they started.

A verdict on footage the tool cannot judge says so in `notes`. A green build on
a scrolling capture would otherwise mean nothing while looking like it meant
something.

## D22 — A shift is measured, not just detected

Detecting content shift was never the hard part: a banner pushing an article
down is one of the largest changes in a recording. The hard part is that
"content appeared" and "content moved" are identical in the statistics — one
transition, one region, persists afterwards — and on a page they are the
difference between the site working and the site being broken.

Telling them apart needs the pixels. The same content in a new place registers
against itself at an offset; new content does not register at any offset. That
is a registration problem, and correlating one-dimensional brightness profiles
solves it at O(size + search) — which suits a page, being rows of text and
stacked blocks with axis-aligned shifts.

Three things fell out of building it:

**Frames are fetched on demand, not retained during the pass.** The first
attempt kept the frames of the largest transitions. That is exactly backwards:
a 2px slide of a card is one of the *smallest* changes in a recording, and
size-based retention threw away precisely the case the feature exists for. A
second decode of the two frames either side is exact, needs no heuristic, and
is measured at full resolution rather than at the analysis width — so the
reported displacement is exact regardless of `--analysis-width`.

**The gate belongs on absolute gain, not relative improvement.** An axis that
did not move starts at almost zero difference, where "50% better" is noise, and
correlating that flat signal invented an 18px sideways move on content that had
only dropped. Measured: a genuine 2px slide gains 0.65 luminance units on its
axis, an axis that did not move gains 0.00. The threshold sits at 0.25, between
them, and a test asserts the separation still holds.

**Two edges are one block.** Content sliding sideways changes only its two
vertical edges, which are correctly separate events and separately unreadable —
two hairlines with no explanation. Brief events sharing an instant are now
tried together, and the defect scenario's 2px card jitter went from two mystery
slivers to one 124x40 block that moved 2px right and back.

## D20 — Sampling a window is a flag, not arithmetic

Both round-3 agents, independently, spent most of their manual iteration
working out step sizes: "I had to hand-pick 8-11 timestamps per event at
0.03-0.05s spacing, guessing at reasonable step sizes". An event's start and
end do not say what a toggle or a drift looks like, and a single still cannot
show either.

`--during start:end --count N` takes an event's own span back and spaces the
samples. `next_steps` proposes one for any event with internal cadence, sized
to about two samples per change so a flicker's tiles land on alternating
states rather than aliasing onto one.

The same round found the suggested still for a `motion` event was taken at its
peak, which for something that exits the frame is the moment it left — the tile
came back blank. Travelling and evolving events are now sampled at their middle.

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
