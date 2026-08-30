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
