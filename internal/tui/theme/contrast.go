package theme

import (
	"image/color"
	"math"
	"strconv"
)

// Luminance is WCAG 2.2 relative luminance, 0 for black and 1 for white.
//
// https://www.w3.org/TR/WCAG22/#dfn-relative-luminance
func Luminance(c color.Color) float64 {
	if c == nil {
		return 0
	}
	r, g, b := channels(c)
	return 0.2126*linearise(r) + 0.7152*linearise(g) + 0.0722*linearise(b)
}

// Ratio is the WCAG contrast ratio between two colours, from 1 (identical) to
// 21 (black on white). It is symmetric.
//
// This function is why the palette in research/17-design-system.md carries a
// number beside every token. Contrast is computed and asserted on every commit,
// never eyeballed: eyeballing it on one good monitor is the documented way
// inaccessible defaults ship.
func Ratio(a, b color.Color) float64 {
	la, lb := Luminance(a), Luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// channels returns the 0–1 sRGB components of c, discarding alpha. Every colour
// labdash renders is opaque, so there is nothing to composite.
func channels(c color.Color) (r, g, b float64) {
	ri, gi, bi, ai := c.RGBA()
	if ai == 0 {
		return 0, 0, 0
	}
	const full = 0xffff
	return float64(ri) / full, float64(gi) / full, float64(bi) / full
}

func linearise(c float64) float64 {
	if c <= 0.03928 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// The two thresholds the product holds itself to. AA is the floor for every
// shipped theme; AAA is what the contrast theme re-derives to reach.
const (
	// RatioAAText is WCAG AA for body text.
	RatioAAText = 4.5
	// RatioAAAText is WCAG AAA for body text.
	RatioAAAText = 7.0
	// RatioUI is WCAG 1.4.11 for a graphical element that identifies a
	// component or its state — a focus ring, an accent fill.
	RatioUI = 3.0
)

// FormatRatio renders a ratio the way the design system writes it: two
// decimals, so 12.06:1 and 4.50:1 line up in a column.
func FormatRatio(r float64) string {
	return strconv.FormatFloat(r, 'f', 2, 64) + ":1"
}
