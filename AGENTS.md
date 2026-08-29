# agent-motion

Temporal-projection CLI for AI agents. Go + Cobra.

## Project rules

- The binary is `agent-motion`; the primary command is `project <video>`.
- Single resources default to pretty JSON. Any future list command defaults to
  NDJSON. `--format json|yaml|jsonl` overrides either default.
- Every failure is one JSON object on stderr with `fixable_by`
  (`agent` | `human` | `retry`) and an actionable `hint` where possible.
- Never treat a temporal projection as video reconstruction or a source of
  ground truth. Its encoding is lossy and must be returned in metadata.
- Decoding is delegated to a locally installed FFmpeg/FFprobe. Do not add a
  network service or a heavyweight ML dependency to v1.
- Preserve pixel coordinates. Do not resize, crop, stabilise, or register
  frames unless a command explicitly requests it and the metadata records it.
- The first mode must remain deterministic for the same decoder version,
  input, flags, and pixel format.
- Keep FFmpeg process execution behind a small package boundary; accumulator
  tests must run without a video file or an installed decoder.

## Verification

```sh
GOCACHE=$(pwd)/.cache/go-build go test ./... -count=1
GOCACHE=$(pwd)/.cache/go-build go vet ./...
golangci-lint run ./...
```

## Keeping docs in sync

When a command, flag, projection channel, output field, or transform changes,
update all of these in the same change:

- `internal/cli/usage_text.go`
- the matching `skills/agent-motion/references/commands/*.md` file
- `skills/agent-motion/SKILL.md` when routing or the core contract changes
- `design-docs/cli-design.md` and `design-docs/behavior-reference.md`
- `README.md` for user-facing changes

## Design references

- `design-docs/initial-design.md` — problem statement, hypothesis, and scope
- `design-docs/architecture.md` — package and process boundaries
- `design-docs/cli-design.md` — command and output contract
- `design-docs/behavior-reference.md` — decoder and projection semantics
