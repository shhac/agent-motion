package cli

const usageText = `agent-motion: tells you what happens in a video over time, as text you can act on.

WHAT IT IS FOR
  Fixed-viewport recordings: UI captures, browser sessions, visual tests,
  rendering and game debugging. It finds where and when pixels change. It does
  not recognise objects, read text, or explain why something changed.

COMMANDS
  inspect <video>    Dimensions, frame rate, duration, codec. Decodes nothing.
  timeline <video>   What changes and when: narrative, events, quiet stretches.
  sheet <video>      One image of many labelled frames. Shows what it looks like.
  project <video>    Timeline plus an activity-map PNG in source coordinates.
  frames <video>     Write real source frames at chosen timestamps.
  usage              This overview. '<command> --help' lists every flag.

THE USUAL PATH
  1. agent-motion timeline clip.mp4         what happened, and when
  2. agent-motion sheet clip.mp4            what it actually looks like
  3. agent-motion timeline clip.mp4 --start 17 --end 19 --threshold 4
                                            zoom into a suspicious range
  4. agent-motion frames clip.mp4 --at 17.6 look at the exact frame

  Every result carries next_steps with commands you can run verbatim.

EVENT KINDS
  cut      most of the frame changes at once and stays changed
  flash    most of the frame changes for a frame or two, then returns
  step     brief localised change that is still there afterwards
  blip     brief localised change that reverts
  flicker  the same area toggles repeatedly; changes_per_second is reported
  motion   activity whose centre travels; direction and distance are reported
  gradual  too slow to see between frames, found over the --drift window
  busy     sustained activity with no clearer shape

  Kinds describe the shape of a change, never its meaning.

SENSITIVITY
  --threshold 12   per-pixel change to ignore, 0..255. Lower finds subtler
                   change and more codec noise. This is the main dial.
  --drift 1        also compare each frame with the frame this long ago, which
                   is the only way a slow fade is seen at all. 0 disables it.
  --analysis-width 320   analysis is downscaled for speed; --native uses the
                   source resolution when small details matter.

OUTPUT
  One JSON object on stdout. --format json|yaml|jsonl overrides the default.
  Read 'narrative' first, then 'events', then 'limits'. The 'limits' field says
  what this run could not have seen.

PROJECT IMAGE
  Each pixel keeps its source x,y. Red is how much it changed, green is when
  (black early, bright late), blue is how often. Black is no change. Whole-frame
  cuts are excluded from the image and listed in transitions_excluded_from_image.
  Gradual events do not appear in it; read them from 'events'.

ERRORS
  One JSON object on stderr: {"error","fixable_by","hint"?}.
  fixable_by=agent: fix the input or flags; human: install or grant something;
  retry: transient decoder or I/O failure.
`
