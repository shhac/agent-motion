package fixture

import "image"

// Layout is the content-shift scenario: an article page where a banner and a
// late image each push everything below them down the page, plus a badge that
// appears without moving anything.
//
// The badge is the control. A tool that reports every one-off change as a shift
// is no more useful than one that reports none, so the scenario contains a
// change that is emphatically not a shift.
func Layout() Scenario {
	return Scenario{
		Name: "layout", draw: drawLayout,
		Width: 800, Height: 600, FPS: 30, Frames: 240,
		Events: []Event{
			{
				Name: "banner-shift", Kind: "shift", Start: 2, End: 2.04,
				Region:      image.Rect(0, 60, 800, 600),
				Description: "A promo banner loads at the top and pushes everything below it down 40px, permanently.",
			},
			{
				Name: "inline-shift", Kind: "shift", Start: 4, End: 4.04,
				Region:      image.Rect(60, 250, 740, 600),
				Description: "An inline image loads mid-article and pushes the text and figure below it down a further 24px.",
			},
			{
				Name: "badge-appear", Kind: "appearance", Start: 6, End: 8,
				Region:      image.Rect(700, 20, 760, 44),
				Description: "A badge appears in the header. Nothing moves — this must not be reported as a shift.",
			},
		},
	}
}

var (
	pagePaper  = rgb{0xff, 0xff, 0xff}
	pageHeader = rgb{0x1f, 0x2d, 0x3d}
	pageBanner = rgb{0xff, 0xd7, 0x66}
	pageTitle  = rgb{0x22, 0x22, 0x22}
	pageBody   = rgb{0x9a, 0x9a, 0x9a}
	pageFigure = rgb{0xcf, 0xd8, 0xe3}
	pageInline = rgb{0xb4, 0xc4, 0xd4}
	pageBadge  = rgb{0x3d, 0xd6, 0x8c}
)

// bannerShift and inlineShift are the two displacements the scenario exists to
// test. Everything below each one moves by exactly this much.
const (
	bannerShift = 40
	inlineShift = 24
)

func drawLayout(s Scenario, dst []byte, index int) {
	t := float64(index) / s.FPS

	top, inline := 0, 0
	if t >= 2 {
		top = bannerShift
	}
	if t >= 4 {
		inline = inlineShift
	}

	fill(dst, s.Width, image.Rect(0, 0, s.Width, s.Height), pagePaper)
	fill(dst, s.Width, image.Rect(0, 0, s.Width, 60), pageHeader)
	if t >= 2 {
		fill(dst, s.Width, image.Rect(0, 60, s.Width, 60+bannerShift), pageBanner)
	}
	if t >= 6 {
		fill(dst, s.Width, image.Rect(700, 20, 760, 44), pageBadge)
	}

	// Title, then body copy, then a figure — all pushed down by whichever
	// shifts have happened above them.
	fill(dst, s.Width, image.Rect(60, 90+top, 520, 130+top), pageTitle)
	for i := 0; i < 3; i++ {
		y := 160 + top + i*26
		fill(dst, s.Width, image.Rect(60, y, 700-(i%3)*90, y+12), pageBody)
	}
	if t >= 4 {
		fill(dst, s.Width, image.Rect(60, 250+top, 420, 250+top+inlineShift), pageInline)
	}
	for i := 3; i < 6; i++ {
		y := 260 + top + inline + (i-3)*26
		fill(dst, s.Width, image.Rect(60, y, 700-(i%3)*90, y+12), pageBody)
	}
	fill(dst, s.Width, image.Rect(60, 350+top+inline, 400, 540+top+inline), pageFigure)
}
