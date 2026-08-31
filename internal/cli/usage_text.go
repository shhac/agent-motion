package cli

const usageText = `agent-motion: tells you what happens in a video over time, as text you can act on.

WHAT IT IS FOR
  Fixed-viewport recordings: UI captures, browser sessions, visual tests,
  rendering and game debugging. It finds where and when pixels change. It does
  not recognise objects, read text, or explain why something changed.

COMMANDS
  inspect <video>    Dimensions, frame rate, duration, codec. Decodes nothing.
  timeline <video>   What changes and when: narrative, events, quiet stretches.
  activity <video>   Where it changes: a grid of busy places, as NDJSON.
  check <video>      Assert conditions and exit non-zero if they fail.
  sheet <video>      One image of many labelled frames. Shows what it looks like.
  project <video>    Timeline plus an activity-map PNG in source coordinates.
  frames <video>     Write real source frames at chosen timestamps.
  compare <video>    Say exactly how two moments differ, and draw it.
  mcp                Serve the same commands over MCP (stdio, or --http addr).
  usage              This overview. '<command> --help' lists every flag.

THE USUAL PATH
  1. agent-motion timeline clip.mp4         what happened, and when
  2. agent-motion sheet clip.mp4            what it actually looks like
  3. agent-motion timeline clip.mp4 --start 17 --end 19 --threshold 4
                                            zoom into a suspicious range
  4. agent-motion frames clip.mp4 --at 17.6 look at the exact frame

WATCHING ONE EVENT UNFOLD
  agent-motion sheet clip.mp4 --during 13.07:13.40 --count 10 --quick
  An event's start and end do not say what a toggle or a drift looks like.
  Paste the event's own span back in and the samples are spaced for you.
  --during works on frames too.

NARROWING DOWN
  agent-motion timeline clip.mp4 --format jsonl | grep '"kind":"shift"'
  jsonl renders the events one per line, with the narrative, suitability and
  limits following as meta lines, so you can filter by kind, time or region
  without parsing the whole document.

WHERE, NOT JUST WHEN
  agent-motion activity clip.mp4
  The activity map in text, for narrowing down without looking at a picture.
  One line per part of the frame that was busy while the rest of it held
  still, busiest first, with the stretches it was busy for. Stretches where
  the whole frame moved at once are reported separately as frame_wide: they
  locate nothing, and would otherwise light every cell equally.

  Sort or filter on busy_share. A cell busy for most of the recording is a
  spinner, a video or an animation; one busy for a fiftieth of it is a single
  change worth a closer look. Pass a box straight to --region:
  agent-motion sheet clip.mp4 --region 480,664,800,800

CONTENT SHIFT
  A 'shift' is content that moved rather than content that appeared, which on
  a page is the difference between a bug and the page working. moved_by_pixels
  is the displacement in source pixels, positive Y down, measured from the two
  real frames either side. layout_shift_score is the share of the frame
  affected times how far it went — CLS-shaped, but not Chrome's CLS, which
  comes from the DOM and knows which elements are unstable.

TESTING A RECORDING
  agent-motion check clip.mp4 --max-shift-score 0.05 --no-stall
  Exits non-zero if an assertion fails, so a visual regression can break a
  build. Every threshold is opt-in; with none given nothing is asserted and
  the result says so. Each failure names the event that broke it.

IS IT THE SAME AS IT WAS?
  agent-motion compare clip.mp4 --at 14.9,18.5
  Returns an exact pixel count, so "it came back" and "it only looks similar"
  stop being a matter of eyeballing two stills. It separates identical from
  merely below the threshold. Add -o diff.png to draw what differs — the only
  way to see a change of a pixel or two.

SEEING SOMETHING SMALL
  A 20x20 indicator is invisible in a 640px still. Crop to it and magnify:
  agent-motion frames clip.mp4 --at 6.2 --region 200,120,202,160 --pad 24 \
    --width 480
  --region takes an event's region_xyxy verbatim. Cropping happens before
  scaling, so --width magnifies the region rather than shrinking the frame.

  Every result carries next_steps with commands you can run verbatim.

EVENT KINDS
  cut      most of the frame changes at once and stays changed
  flash    most of the frame changes for a frame or two, then returns
  step     brief localised change that is still there afterwards
  blip     brief localised change that reverts
  flicker  the same area toggles repeatedly; changes_per_second is reported
  motion   activity whose centre travels; direction and distance are reported
  shift    the same content in a new place; moved_by_pixels says how far
  gradual  too slow to see between frames, found over the --drift window
  busy     sustained activity with no clearer shape
  stall    activity that was running continuously stopped, then resumed. This
           is an absence of change, so it is the one finding no pixel shows.

  Kinds describe the shape of a change, never its meaning.

SENSITIVITY
  --threshold 12   per-pixel change to ignore, 0..255. Lower finds subtler
                   change and more codec noise. This is the main dial.
  --drift 1        also compare each frame with the frame this long ago, which
                   is the only way a slow fade is seen at all. 0 disables it.
  --analysis-width 320   analysis is downscaled for speed; --native uses the
                   source resolution when small details matter.

ACTIVITY SPARKLINE
  One character per time bucket, from least to most active: _ . : - = + * #
  Scaled against activity_sparkline_full_scale, and frame-to-frame only, so
  gradual events do not appear in it. Orientation, not measurement.

OUTPUT
  One JSON object on stdout. --format json|yaml|jsonl overrides the default.
  Read 'narrative' first, then 'events', then 'limits'. The 'limits' field says
  what this run could not have seen.

DID IT FINISH?
  settled_at_seconds        when anything last changed
  layout_settled_at_seconds when the *content* last changed
  A ticker or spinner keeps the first late forever while the page has been
  stable for seconds. For "has this finished loading", read the second.
  Events marked continuous run steadily in one small fixed place for much of
  the interval — the shape of animation, not of a fault.

  A cut also reports uniform_shade_change. True means the whole frame changed
  brightness together and the content underneath is unchanged: an overlay, a
  dim or a theme switch, not a new screen. shade_scale near 0.5 means dimmed
  to half.

SUITABILITY
  Every analysis reports 'suitability'. When the verdict is not "suitable",
  most of the frame is moving at once — a camera pan, a scroll, full-motion
  video — and the event boundaries are arbitrary. The narrative leads with that
  warning. Use 'sheet' on such footage instead of trusting the event list.

PROJECT IMAGE
  Each pixel keeps its source x,y. Red is how much it changed, green is when
  (black early, bright late), blue is how often. Black is no change. Whole-frame
  cuts are excluded from the image and listed in transitions_excluded_from_image.
  The legend band and omitted_from_image name everything the picture leaves out
  — read that before concluding nothing happened somewhere.

ERRORS
  One JSON object on stderr: {"error","fixable_by","hint"?}.
  fixable_by=agent: fix the input or flags; human: install or grant something;
  retry: transient decoder or I/O failure.
`
