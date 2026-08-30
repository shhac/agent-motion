# Capturing real page recordings

Instructions for an agent or tool with a browser and a screen recorder. The
purpose is to test `agent-motion` against real web pages instead of synthetic
fixtures — specifically against **content shift**: a banner, image, ad or font
loading late and pushing the page around.

You do not need `agent-motion` itself. Just produce the recordings.

## Where to put them

```
/Users/paul/projects-personal/agent-motion/.cache/eval/real/
```

Create it if missing. It is git-ignored, so nothing you write there is
committed. One `.mp4` per capture, plus one `.md` sidecar of the same name.

Name files `<site>-<scenario>.mp4`, lowercase, hyphenated:

```
forbes-load.mp4        forbes-load.md
guardian-load.mp4      guardian-load.md
wikipedia-load.mp4     wikipedia-load.md
bbc-nav.mp4            bbc-nav.md
guardian-scroll.mp4    guardian-scroll.md
```

## Recording settings — the part that matters

Get these wrong and the recording is unusable. They are not stylistic.

| Setting | Value | Why |
|---|---|---|
| **Frame rate** | **30 fps constant (CFR)** — 60 if easy | Variable frame rate is the single worst failure. Screen recorders default to VFR, which drops frames when the screen is still — exactly the stillness the analysis measures. Every timestamp then drifts. If your recorder only does VFR, transcode after: `ffmpeg -i in.mp4 -fps_mode cfr -r 30 -c:v libx264 -crf 18 -pix_fmt yuv420p out.mp4` (on ffmpeg 6 and older, `-vsync cfr` instead of `-fps_mode cfr`) |
| **Region** | **The page viewport only** | Not the whole desktop, ideally not browser chrome. A loading spinner in the tab, a blinking cursor in the URL bar, or a menu bar clock are all "activity" and will be reported as findings. |
| **Window size** | **1280×800**, fixed, and never resized mid-capture | Coordinates in the results are source pixels. A resize invalidates every one. |
| **Cursor** | **Do not capture it, or park it in a corner and do not move it** | A moving cursor is genuine on-screen motion and will dominate the results. |
| **Scrolling** | **None during a `-load` capture** | The analysis assumes a fixed viewport. Scrolling makes the whole frame change at once, and the tool will correctly report the recording as unsuitable — which tells us nothing about the page. |
| **Cache** | **Cold. Fresh profile, incognito, or cache disabled** | Layout shift is overwhelmingly a first-load phenomenon. A warm cache hides the thing we are looking for. |
| **Format** | mp4, H.264, `yuv420p` | Widely decodable. Avoid HEVC and VFR webm. |
| **Quality** | High — CRF 18 or better | Compression noise is real change. Heavy compression buries a 2px shift. |

**Timing.** Start recording **before** navigating and stop about **10 seconds
after** the page settles. The moment of navigation must be inside the
recording — a capture that starts on an already-loading page loses the event.
Roughly: start recorder → wait 1s → navigate → wait 10s → stop.

**Don't interact** during a `-load` capture. No clicking, no dismissing the
cookie banner, no scrolling. The banner appearing *is* one of the shifts we
want to see. Dismiss it afterwards if a later capture needs it gone.

## What to capture

Aim for eight to twelve recordings. The controls matter as much as the
offenders — a tool that flags everything is as useless as one that flags
nothing.

### Likely to shift

| File | URL | What to do |
|---|---|---|
| `forbes-load.mp4` | https://www.forbes.com | Load the front page. Known for a late interstitial and heavy ad injection. |
| `dailymail-load.mp4` | https://www.dailymail.co.uk | Load the front page. Very ad-dense, many late images. |
| `cnn-load.mp4` | https://www.cnn.com | Load the front page. Ad slots that reserve space poorly. |
| `allrecipes-load.mp4` | https://www.allrecipes.com | Load any recipe page. Recipe sites are the classic ad-driven-reflow case. |
| `espn-load.mp4` | https://www.espn.com | Load the front page. Live score modules inject late. |
| `weather-load.mp4` | https://weather.com | Load the front page. Widgets and ads. |
| `independent-load.mp4` | https://www.independent.co.uk | Load the front page. Ad-heavy, consent banner. |
| `guardian-load.mp4` | https://www.theguardian.com | Load the front page. Custom web fonts — watch for a font swap reflowing text. |

### Controls — expected to be stable

| File | URL | What to do |
|---|---|---|
| `wikipedia-load.mp4` | https://en.wikipedia.org/wiki/Layout | Load the article. Should be nearly still after first paint. |
| `hackernews-load.mp4` | https://news.ycombinator.com | Load the front page. Tiny, static, essentially no shift. |

### Two deliberate edge cases

| File | URL | What to do |
|---|---|---|
| `bbc-nav.mp4` | https://www.bbc.co.uk/news | Load, wait for it to settle, **then click one headline** and let the article load. Tests a navigation rather than a cold load. Note in the sidecar roughly when you clicked. |
| `guardian-scroll.mp4` | https://www.theguardian.com | Load, wait to settle, then scroll slowly down for ~5s. This one *should* be reported as unsuitable — it is a check that the tool refuses to over-read a moving viewport. |

Substitute freely if a site is unreachable or asks for a login. Any ad-supported
news, recipe or sports page is a fine stand-in. Only public pages, no accounts,
no personal data on screen.

## The sidecar

Next to each `.mp4`, a `.md` of the same name. Six lines is enough — but be
accurate, because this is the only ground truth these recordings will have.

```markdown
url: https://www.forbes.com
captured: 2026-08-30T14:05Z
viewport: 1280x800
fps: 30 (CFR)
recorder: <tool and settings>
navigation_at: ~1.2s into the recording
observed: The cookie banner appeared at the bottom around 1.5s. The headline
  block jumped down roughly one line when a top ad slot filled, maybe 2s in.
  A hero image popped in later and pushed the article list down again.
```

`observed` is the important one. Write what you actually *saw* move, roughly
when, and roughly where on screen — even vaguely ("something below the header
jumped down about a line"). Uncertainty is fine and useful; a guess presented
as fact is not. If you genuinely saw nothing shift, say that — a clean
recording of a stable page is a real result.

## What makes a recording useless

Listed in rough order of how often it happens:

1. **Variable frame rate.** Check with
   `ffprobe -v error -select_streams v:0 -show_entries stream=r_frame_rate,avg_frame_rate -of default=nw=1 file.mp4`.
   If `r_frame_rate` and `avg_frame_rate` differ noticeably, transcode to CFR.
2. **A moving mouse cursor**, which becomes the loudest thing in the video.
3. **Scrolling during a `-load` capture.**
4. **Whole-desktop capture** — menu bar clocks, dock animations, notifications.
5. **Starting the recording after navigation has begun**, losing the load.
6. **A warm cache**, which hides the shift entirely.
7. **Heavy compression**, which buries small shifts in noise.

A recording that fails any of the first five is worth redoing rather than
sending. If you are unsure whether one is good, send it and say so in
`observed`.
