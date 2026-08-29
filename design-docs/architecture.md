# Architecture

```text
cmd/agent-motion
  → internal/cli                 Cobra command tree + shared agent CLI flags
      → internal/projection      FFprobe metadata, FFmpeg raw RGB stream,
                                  deterministic accumulator, PNG write
      → lib-agent-output         JSON/NDJSON/YAML and structured errors
```

`internal/projection` is split conceptually into two seams:

- process boundary — invokes locally installed FFprobe/FFmpeg and streams
  `rgb24` frames
- pure accumulator — consumes same-sized RGB frames and calculates activity
  statistics; tests do not depend on FFmpeg or fixture media

This boundary makes future decoder replacement, synthetic tests, and exact
benchmark fixtures straightforward. It also keeps the CLI free of video
details.

The first implementation keeps decoded frames one at a time, plus per-pixel
statistics and the prior frame. Memory is therefore O(width × height), rather
than O(frames × width × height). Sorting the active-pixel magnitudes for p99
normalization is also O(active pixels) memory; profile that before introducing
approximate quantiles.

No credential storage, network client, cache, or MCP surface exists in v1.
Those are not inherited merely because sibling `agent-*` tools use them.
