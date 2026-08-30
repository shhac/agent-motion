# agent-motion

Tells an agent what happens in a video over time. Go + Cobra.

## What this tool is

The primary output is a **described timeline** — events with kinds, timestamps
and regions — not an image. Images (`sheet`, `project`) support that answer;
they are not the product. See `design-docs/vision.md`.

## Project rules

- The binary is `agent-motion`. `timeline` is the primary command; `project` is
  `timeline` plus the activity-map image. `compare` stands outside the ladder:
  it answers a question about two specific moments.
- Options take zero literally. A threshold of zero, a drift of zero and zero
  buckets are all meaningful settings, so `withDefaults` fills in only negative
  values and `engine.Defaults` is how a caller gets the documented ones.
- Single resources default to pretty JSON. Any future list command defaults to
  NDJSON. `--format json|yaml|jsonl` overrides either default.
- Every failure is one JSON object on stderr with `fixable_by`
  (`agent` | `human` | `retry`) and an actionable `hint` where possible.
- Every analysis result must carry `limits`, `next_steps` and `suitability`. A
  caller must never be able to read "no events" as "nothing happened", nor read
  events from unsuitable footage as findings.
- An image must state what it leaves out, in the image. Disclosing it only in
  metadata is how a reader concludes nothing happened where plenty did.
- Event kinds name the *shape* of a change, never its meaning. Do not add a
  kind that asserts what something is.
- Never present an activity image as a reconstruction or as ground truth. Its
  encoding is lossy and is returned in `encoding`.
- Preserve pixel coordinates. Do not resize, crop, stabilise or register frames
  unless a command explicitly requests it and the result records it. The
  projection legend is appended *below* the frame for this reason.
- Decoding is delegated to a locally installed FFmpeg/FFprobe. No network
  service and no heavyweight ML dependency.
- All process execution stays inside `internal/video`. `internal/motion` and
  `internal/render` must remain pure, and the test suite must keep running with
  no FFmpeg installed and no media on disk.
- Analysis is deterministic for the same decoder version, input, flags and
  pixel format.

The skill is how an agent finds this tool at all, so `internal/skillmeta` guards
its frontmatter and its links. A malformed skill fails silently — the tool is
simply never used, with nothing to say why.

## Verification

```sh
GOCACHE=$(pwd)/.cache/go-build go test ./... -count=1
GOCACHE=$(pwd)/.cache/go-build go vet ./...
golangci-lint run ./...
```

`make fixtures` renders the evaluation videos (`make fixture SCENARIO=player`
for one). They are the only thing here that needs FFmpeg; the test suite does
not, and must not start to.

## Keeping docs in sync

When a command, flag, event kind, output field or transform changes, update all
of these in the same change:

- `internal/cli/usage_text.go`
- `skills/agent-motion/SKILL.md` and `skills/agent-motion/references/*.md`
- `design-docs/cli-design.md` and `design-docs/behavior-reference.md`
- `design-docs/decisions.md` when the change was forced by something learned
- `README.md` for user-facing changes

## Design references

- `design-docs/vision.md` — the problem, the goal, and the scope
- `design-docs/architecture.md` — packages, the decoder seam, memory
- `design-docs/cli-design.md` — command and output contract
- `design-docs/behavior-reference.md` — detection and image semantics
- `design-docs/evaluation.md` — the reference scenarios and how they are scored
- `design-docs/agent-trials.md` — what agents given only the tool actually said
- `design-docs/decisions.md` — what changed and what forced it
- `design-docs/archived/` — superseded documents, kept for the reasoning
