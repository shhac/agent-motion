package cli

const usageText = `agent-motion: deterministic temporal projections for video debugging agents.

COMMANDS
  project <video>  Decode an interval, write one PNG, and print JSON metadata.
  usage            This overview. Run 'agent-motion project --help' for flags.

PROJECT
  agent-motion project recording.mp4 --start 12 --end 18
  Default output: recording.temporal.png. --output selects another PNG path.
  --threshold (default 12) suppresses compression/noise-scale RGB changes.
  --end omitted means decode to the end of the source video.

ENCODING (v1 change mode)
  R = accumulated change magnitude, normalized within the selected interval.
  G = mean normalized time of detected change: dark early, bright late.
  B = change frequency plus sign reversals; repeated oscillation rises in blue.
  Black means no detected change. This is a lossy activity map, not a frame.

OUTPUT
  project writes a PNG and emits one JSON object by default. It records the
  exact channel encoding, selected interval, decoded frame count/FPS, motion
  coverage, peak activity time, threshold, and decoder names. --format
  json|yaml|jsonl overrides (json is the single-resource default).

ERRORS
  One JSON object on stderr: {"error","fixable_by","hint"?}.
  fixable_by=agent: correct input; human: install/provide a dependency;
  retry: transient decoder/I/O failure.

TEMPORAL ZOOM
  Start broad, inspect the projection, then recurse into an interesting range:
  0-30s → 17-19s → 17.5-17.9s → individual frames when necessary.
`
