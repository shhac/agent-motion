# What agent-motion is for

## The problem

An image-capable model can look at a frame. It cannot look at a video. The
usual workaround — sample N frames into an atlas — is expensive and blind: most
frames of a screen recording are identical, so the budget is spent re-reading a
static screen, and whatever happened between two samples is simply gone.

The question an agent actually has is temporal: *when did this break, and
where on screen?* That question has a cheap answer, and the answer is mostly
text.

## The goal

Let an agent find out what happens in a video over time, accurately enough to
act on, without watching it.

Concretely, for a fixed-viewport recording, one command should say: this is
28 seconds long; something moved left to right across the middle from 2 to 5
seconds; a badge appeared bottom-right at 6.5 and stayed; a panel flickered at
10 changes a second from 9 to 12; there were hard cuts at 15 and 18; a single
white frame at 21; and something faded in slowly in the lower centre from 23.

That is a description a model can reason about, at a fraction of the cost of
the frames it summarises.

## The shape of the answer

Four things, in the order an agent needs them:

1. **When** — a described timeline of events with timestamps.
2. **Where** — a region in source pixel coordinates for each event.
3. **What kind** — the shape of each change: cut, flash, step, blip, flicker,
   motion, gradual, busy. Shape only; never meaning.
4. **What it looks like** — real frames, when the agent decides it needs them.

Text carries the first three. Only the fourth needs pixels, and by then the
agent knows which pixels to ask for.

## Scope

**In scope.** Fixed-viewport recordings: UI captures, browser sessions, visual
regression runs, rendering and game debugging. Local decoding through an
installed FFmpeg. Deterministic, inspectable, cacheable output.

**Out of scope.** Object recognition, text reading, semantic interpretation,
optical flow, camera stabilisation, audio, any network service, and any
model-specific video-token pipeline. The tool reports where and when pixels
changed. What that means is the agent's job, and every result says so.

One consequence is worth stating, because an evaluation agent asked for it
directly: the tool cannot tell a camera pan from wind in the grass. Both make
the whole frame change continuously, and separating them needs optical flow.
What it can do is notice that it cannot help, and say so — which is what the
suitability verdict is for.

## Why this is not just frame differencing

Frame differencing is the primitive, not the product. The product is what sits
on top of it:

- **Two timescales.** Adjacent-frame differencing cannot see a fade — a change
  slower than the threshold per frame is invisible to it, permanently. A second
  comparison against a delayed frame is the only thing that catches it.
- **Segmentation and classification.** A series of per-frame numbers is not an
  answer. Grouping it into events and naming their shape is.
- **Stated limits.** Every result carries what that run could not have seen, so
  "nothing found" cannot be misread as "nothing happened".
- **A next move.** Every result proposes the command that would narrow the
  question.

## The temporal zoom loop

```text
timeline over the whole recording
  → an event range worth a closer look
  → timeline over that range at a lower threshold
  → a contact sheet of those moments
  → individual frames
```

Each step costs more than the last and is entered only when the previous one
justifies it.
