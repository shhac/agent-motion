# Agent trials

The tool is for agents, so it is tested by giving agents a video and no context
and asking what they can tell.

## Method

Each trial runs in a scratch directory containing only the binary, the skill,
and one video. No repository, no source, no ground truth, no design docs — a
trial that can read the answer is not a trial. Agents are told the directory is
the whole world and asked for two things: what happens in the video, and an
honest critique of using the tool, including a rating and what would move it.

Three videos, chosen to cover the range:

| Trial | Video | Point |
|---|---|---|
| describe | the reference scenario, renamed | can an agent reconstruct known events? |
| debug | the defect scenario, given as a "the dashboard felt janky" ticket | can an agent diagnose from a symptom? |
| footage | ten seconds of a Creative Commons animated film | does the tool stay honest on footage it is bad at? |

## Round 1

### What went right

All three agents reached the right answer about *what happened*. The describe
trial reconstructed all seven events in the reference video with correct times,
colours and screen positions, and correctly separated what it had verified from
what it was inferring. The debug trial found all three planted faults. Both
rated first contact as frictionless: the skill's opening command worked, and
every `next_steps` command ran verbatim.

The contact sheet was repeatedly the thing that turned abstract regions into
understanding. One agent: "without it I'd have no idea what anything looked
like."

### What went wrong, and what changed

**The freeze read as "nothing to see here."** The debug trial's headline
finding — a 3.2 second full stall, the thing that actually explains "felt
janky" — came out of the tool only as `quiet_ranges: [[10.83, 14]]` and the
sentence "Nothing changes during 10.83s-14.00s". The agent's verdict: the
tool's own vocabulary "actively undersells it — 'quiet' reads as 'fine', not
'the screen froze'". It had to re-run at a lower threshold and diff frames with
ImageMagick to confirm what the tool already knew. → `stall` is now an event
kind, detected as continuously running activity that stops and resumes in the
same place, and the narrative says so in its own sentence. See D12.

**Small findings could not be looked at.** Both trials left the tool for
ImageMagick to crop, because a 20x20 indicator or a 2px card edge is invisible
in a 640px still. "For a tool whose whole pitch is small regions of change, not
being able to say 'give me this region, zoomed' is a real gap." → `--region`
and `--pad` on `frames` and `sheet`, cropping before scaling so a small region
comes back magnified. `next_steps` now proposes a crop for the smallest event
found. See D13.

**The activity image hid its own omissions.** The describe trial found that
`project` silently dropped four of seven events — every cut, the flash and the
gradual fade — disclosed only in a JSON field. "Glancing at the picture alone
gives the false impression nothing happens after ~12s." → the legend band now
names what the image leaves out, in the picture, and the result carries the
same sentence. See D14.

**A blank tile and a white frame looked identical.** Landing on the
single-frame white flash produced a tile the agent could not distinguish from a
broken thumbnail without extracting the frame separately. → every tile now has
a hairline border.

**Tiles lost their labels when you chose the timestamps.** With `--at`, the
sheet had no analysis and so no event labels, forcing manual cross-referencing
— "the one place a tool built around 'don't make the agent guess' still made me
guess-and-check." → the sheet analyses even when given timestamps, so tiles are
always labelled; `--quick` opts out.

**`next_steps` could propose the command you had just run.** A narrowed
timeline suggested itself again, "a no-op loop… would confuse a less careful
agent". → suggestions are checked against the current run.

**The footage trial rated it 4/10 and was right to.** Ten seconds of foliage in
wind produced fourteen confident localised events that were fragments of
shimmering grass, and nothing in the output said so. Worse, the documented
"wrong tool" list named only panning and scrolling, so a reader following the
docs would not have been warned. The agent's ask was exactly right: "a
self-reported fit/confidence heuristic computed and stated up front". → every
analysis now reports `suitability`, the narrative leads with the warning, and
the wrong-tool guidance covers ambient motion and slow zooms. See D10.

**Smaller corrections.** The sparkline's character ramp was undocumented; the
sheet re-embedded the whole timeline even when the caller had just run one; the
docs claimed gradual events "never" appear in the activity image, where an
agent observed edge pixels that occasionally do.

### Scores

| Trial | Round 1 | Main reason given |
|---|---:|---|
| describe | 8/10 | unlabelled sheet tiles; the image silently dropping half the clip |
| debug | 7/10 | the freeze buried in neutral language; no region crop |
| footage | 4/10 | confident events on footage where they mean nothing, with no warning |

The footage score is the one that mattered most: the tool was not wrong about
the pixels, it was wrong about how much its findings were worth, and it said
nothing about the difference.

## Round 2

The same three tasks, run against the tool after round 1's changes, by fresh
agents with no knowledge of the first round.

### What the changes did

**The freeze is now free.** Where round 1 had to re-run at a lower threshold and
diff frames with ImageMagick to establish the stall, round 2 reported: "the tool
found it, named it correctly, and even editorialized helpfully… I only had to
confirm visually; no inference needed."

**The suitability warning lands.** The footage trial: "not buried — it's the
literal first thing you read if you follow the docs' own instruction to read
`narrative` first, and it's also its own top-level structured field, not
something you have to infer. I would have noticed this even skimming." It still
rated the tool 4/10 *for that video*, which is correct and now honest: "it
scores those 4 points honestly… it fails loudly and early instead of quietly
returning garbage."

It also spot-checked the events it had been warned about and confirmed the
warning was right — a `motion` event with a direction and a travel distance
turned out to be leaf shimmer. Without the warning it says it "could easily have
reported 'a bird flies left to right at 8.3s'".

**Region cropping is used and works.** Both trials reached for `--region --pad
--width` unprompted. One called the design "one real design win after another
(crop-before-scale, `region_xyxy` round-tripping directly from events into the
next command, `quiet_ranges` bounding my search space immediately)".

### What round 2 asked for

**Comparing two moments — asked for by three of the six agents across both
rounds, independently.** The describe trial wanted to know whether the scene
after a cut was the same one as before it; the debug trial wanted to confirm
what moved in a two-pixel jitter it could not see by eye. Both named it as the
single biggest gap, in the same words: a `compare`/`diff` between two arbitrary
timestamps. That is now a command. See D15.

**Smaller.** The `frames` docs did not mention that it takes `--dir` rather than
`-o`, or cross-reference `sheet --region` for several moments side by side. A
tile falling inside two events was labelled with only the first, so the end of a
flicker and the start of the stall that interrupted it looked like one thing. A
missing file surfaced FFprobe's stderr as the error text — "the one place the
tool's usually-crisp voice slipped into a subprocess's voice".

**Declined for now.** Both trials wanted the activity image to include stalls
and gradual events. It cannot: a stall has no pixels, and a gradual change never
clears the frame-to-frame threshold that the image is drawn from. Naming the
omission in the legend is the honest answer, and `compare` now covers the case
that drove the request. The footage trial also wanted the tool to separate a
camera pan from ambient motion, which needs optical flow — out of scope, and
recorded in the vision doc as such.

### Scores

| Trial | Round 1 | Round 2 |
|---|---:|---:|
| describe | 8/10 | 8/10 |
| debug | 7/10 | 7/10 |
| footage | 4/10 | 4/10 |

The numbers held while the reasons changed completely, which is the useful
result. Round 1's deductions were about the tool failing to say what it knew;
round 2's are about capabilities it does not have — comparing two moments, since
built, and reconstructing motion, which is out of scope by design. The footage
score stayed at 4 because that video is genuinely the wrong input; what changed
is that the agent now says the tool "fails loudly and early instead of quietly
returning garbage", and would recommend running it first on unfamiliar footage
purely for the suitability gate.

## Round 3

A new scenario the detection rules had never seen — a video player whose
progress bar advances for the whole recording, with three faults hiding behind
it — plus the reference clip again, to check nothing had regressed.

### What the changes did

**Every fault in an unseen scenario was found.** The player trial reported all
three: the backwards jump at 8.0s (straight from `jump_backwards_pixels`), the
flicker at 13.07s, and the caption shift at 17.0s. It called the docs "unusually
good at pre-empting misreadings (suitability, limits, stall semantics)".

**`compare` is used, unprompted, for exactly what it is for.** The describe
trial pixel-verified that the badge was unchanged between 14.0s and 27.9s
("byte-identical") and that the white flash left no trace ("identical, 0 changed
pixels"). Round 2's biggest gap became round 3's routine verification step.
Neither trial reached for an external image tool at any point — "never had to
leave the tool".

### What round 3 asked for

**Sampling a window, again from both agents independently.** "I had to
hand-pick 8-11 timestamps per event at 0.03-0.05s spacing, guessing at
reasonable step sizes… this alone accounted for most of my manual iteration."
An event's start and end do not say what a toggle or a drift looks like, and a
single still cannot show either. `--during start:end --count N` now takes an
event's own span back. See D20.

**A suggested still landed on nothing.** The recommended sheet sampled the
`motion` event at its peak, which for an object leaving the frame is the moment
it left, so that tile came back blank — "I had to build a second, hand-cropped
sheet just to see the object the tool had already detected". Travelling and
evolving events are now sampled at their middle.

**Threshold sensitivity was silent.** Lowering `--threshold` from 12 to 4 did
not only add detail: it re-scoped an event, and the agent only noticed by
diffing two runs. `limits` now says that boundaries depend on the threshold and
the analysis width, and that two runs are not directly comparable.

**Declined.** A numeric colour readout per region, so a colour ramp is legible
from JSON alone. It is a reasonable ask, and it edges toward describing content
rather than change; `--during` plus a crop answers the same question with an
image, which is the honest form of the answer.

### Scores

| Trial | Round 1 | Round 2 | Round 3 |
|---|---:|---:|---:|
| describe | 8/10 | 8/10 | 8/10 |
| debug | 7/10 | 7/10 | — |
| player (new scenario) | — | — | 7/10 |
| footage | 4/10 | 4/10 | — |

Held steady across three rounds while the substance of the complaints moved
each time: round 1 was about the tool not saying what it knew, round 2 about a
capability it lacked, round 3 about ergonomics on top of findings it now
delivers reliably. Both round-3 agents reached a complete, pixel-verified
account without leaving the tool, and the remaining deductions are about
sampling ergonomics and about the things it says plainly that it does not do —
reading text and recognising objects.
