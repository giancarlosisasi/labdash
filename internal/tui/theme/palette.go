package theme

// This file is the only place in labdash where a colour is named.
//
// THM-01.T1 walks every other package and fails the build on a hex value or an
// ANSI index found outside it. The rule only holds because it is enforced.
//
// The palette is warm amber on cool slate — a nod to GitLab's tanuki without
// using GitLab's brand orange, and distinct from the blue-purple every other
// developer tool defaulted to. research/17-design-system.md §2.

// A tone is one semantic role as its author wrote it: a 24-bit value, and the
// ANSI index the sixteen-colour tier uses in place of a nearest match.
//
// The index is declared rather than computed because at that tier the user's
// own terminal theme supplies the hue, and "closest to amber" is a judgement
// (ANSI 3) rather than an arithmetic result. research/17 §2.4.
type tone struct {
	hex  string
	ansi int // noANSI means: no colour at this tier, the terminal's default wins
}

// noANSI marks a token that has no sixteen-colour form. Backgrounds are the
// whole set: a 16-colour terminal renders a background reliably only as reverse
// video, so the selected row uses that and the other surfaces use nothing.
const noANSI = -1

// The sixteen ANSI indices, named so the palette below reads as a decision
// rather than as magic numbers.
const (
	ansiBlack = iota
	ansiRed
	ansiGreen
	ansiYellow
	ansiBlue
	ansiMagenta
	ansiCyan
	ansiWhite
	ansiBrightBlack
	ansiBrightRed
	ansiBrightGreen
	ansiBrightYellow
	ansiBrightBlue
	ansiBrightMagenta
	ansiBrightCyan
	ansiBrightWhite
)

// A palette is one shipped theme as authored, before any tier resolution.
type palette struct {
	name string
	// pinnedTier is the tier this theme is always rendered at, whatever the
	// terminal reports. It is set for ansi and mono, whose entire purpose is to
	// stay at one tier; it is TierUnset for the rest.
	pinnedTier Tier
	// textMin is the contrast every text and meaning token must reach against
	// every surface. Zero means the colours come from the user's terminal and
	// cannot be computed here, and then contrastReason must say so.
	textMin        float64
	contrastReason string

	tones map[Token]tone
}

// ember is the default. Dark, warm amber on cool slate.
//
// Every text and meaning token clears 4.5:1 against all four surfaces, not only
// against bg.base. The pair that decided the palette is text.muted on
// bg.selected — a timestamp on the selected row, the most scanned cell in the
// product — which the published palette missed at 4.00:1.
var ember = palette{
	name:    "ember",
	textMin: RatioAAText,
	tones: map[Token]tone{
		BgBase:     {"#0F1117", noANSI},
		BgSurface:  {"#161922", noANSI},
		BgOverlay:  {"#1E222D", noANSI},
		BgSelected: {"#232838", noANSI},

		BorderFaint:   {"#262B38", ansiBrightBlack},
		BorderDefault: {"#39404F", ansiBrightBlack},
		BorderFocus:   {"#E8A33D", ansiYellow},

		TextPrimary:   {"#E6E9EF", ansiBrightWhite},
		TextSecondary: {"#A8B0C0", ansiWhite},
		TextMuted:     {"#8A94A7", ansiBrightBlack},
		TextInverted:  {"#0F1117", ansiBlack},

		AccentPrimary:   {"#E8A33D", ansiYellow},
		AccentSecondary: {"#7AA2F7", ansiBlue},

		StatusSuccess: {"#7BD88F", ansiGreen},
		StatusWarning: {"#E8C55D", ansiYellow},
		StatusError:   {"#F07178", ansiRed},
		StatusRunning: {"#5FD7E0", ansiCyan},
		StatusPending: {"#C4A7F5", ansiMagenta},
		StatusNeutral: {"#8A94A7", ansiBrightBlack},

		DiffAdded:   {"#7BD88F", ansiGreen},
		DiffRemoved: {"#F07178", ansiRed},
	},
}

// emberLight is the same token names at different values, for a light terminal.
// Verified by the same computation.
var emberLight = palette{
	name:    "ember-light",
	textMin: RatioAAText,
	tones: map[Token]tone{
		BgBase:     {"#FBFBFD", noANSI},
		BgSurface:  {"#F2F3F7", noANSI},
		BgOverlay:  {"#FFFFFF", noANSI},
		BgSelected: {"#E4E8F2", noANSI},

		BorderFaint:   {"#E2E5EC", ansiBrightBlack},
		BorderDefault: {"#C3C9D6", ansiBrightBlack},
		BorderFocus:   {"#8A4F00", ansiYellow},

		TextPrimary:   {"#14161C", ansiBlack},
		TextSecondary: {"#454C5C", ansiBrightBlack},
		TextMuted:     {"#5F6778", ansiBrightBlack},
		TextInverted:  {"#FBFBFD", ansiBrightWhite},

		AccentPrimary:   {"#8A4F00", ansiYellow},
		AccentSecondary: {"#2B58BC", ansiBlue},

		StatusSuccess: {"#106B36", ansiGreen},
		StatusWarning: {"#7A5600", ansiYellow},
		StatusError:   {"#B32E38", ansiRed},
		StatusRunning: {"#0C6169", ansiCyan},
		StatusPending: {"#603AAB", ansiMagenta},
		StatusNeutral: {"#5F6778", ansiBrightBlack},

		DiffAdded:   {"#106B36", ansiGreen},
		DiffRemoved: {"#B32E38", ansiRed},
	},
}

// contrast is AAA, and it is genuinely re-derived rather than ember with more
// saturation.
//
// The surfaces are near-black with almost no luminance between them, because
// 7:1 for every hue is not reachable above a slate background — ember's own
// bg.selected already costs two points of ratio. The hues are lifted and
// desaturated to sit above 7:1 on all four.
var contrast = palette{
	name:    "contrast",
	textMin: RatioAAAText,
	tones: map[Token]tone{
		BgBase:     {"#000000", noANSI},
		BgSurface:  {"#0A0A0C", noANSI},
		BgOverlay:  {"#121216", noANSI},
		BgSelected: {"#1A1A20", noANSI},

		BorderFaint:   {"#2C2C36", ansiBrightBlack},
		BorderDefault: {"#5B6172", ansiWhite},
		BorderFocus:   {"#FFC661", ansiBrightYellow},

		TextPrimary:   {"#FFFFFF", ansiBrightWhite},
		TextSecondary: {"#D6DAE3", ansiBrightWhite},
		TextMuted:     {"#AEB5C2", ansiWhite},
		TextInverted:  {"#000000", ansiBlack},

		AccentPrimary:   {"#FFC661", ansiBrightYellow},
		AccentSecondary: {"#93BBFF", ansiBrightBlue},

		StatusSuccess: {"#5FE08A", ansiBrightGreen},
		StatusWarning: {"#FFD966", ansiBrightYellow},
		StatusError:   {"#FF9AA0", ansiBrightRed},
		StatusRunning: {"#66E3EC", ansiBrightCyan},
		StatusPending: {"#D3B8FF", ansiBrightMagenta},
		StatusNeutral: {"#AEB5C2", ansiWhite},

		DiffAdded:   {"#5FE08A", ansiBrightGreen},
		DiffRemoved: {"#FF9AA0", ansiBrightRed},
	},
}

// ansiTheme defers to the user's own terminal palette entirely, at every tier.
// For somebody with a colour scheme they love and no wish for ours.
//
// Its contrast cannot be computed here and that is not a gap: ANSI index 1 is
// whatever the user's terminal says it is, and asserting a ratio against a
// value we do not know would be an assertion in name only.
var ansiTheme = palette{
	name:           "ansi",
	pinnedTier:     ANSI,
	textMin:        0,
	contrastReason: "Every colour comes from the user's own terminal palette, so no ratio exists to compute. The vocabulary still carries a glyph and a word for every state, which is what keeps it readable.",
	tones:          ember.tones,
}

// mono emits no colour at all. For screenshots, printing and serial consoles.
var mono = palette{
	name:           "mono",
	pinnedTier:     Monochrome,
	textMin:        0,
	contrastReason: "No colour is emitted, so there is no pair to measure. Hierarchy comes from the glyph, the word, weight and layout — which is the monochrome-first rule working as designed.",
	tones:          ember.tones,
}

// palettes is the shipped set. A theme absent from this map cannot be selected,
// and a theme present in it is asserted by the contrast test — the test walks
// this map rather than a list beside it, so a theme cannot ship by being
// forgotten.
var palettes = map[string]palette{
	ember.name:      ember,
	emberLight.name: emberLight,
	contrast.name:   contrast,
	ansiTheme.name:  ansiTheme,
	mono.name:       mono,
}

// Names is every shipped theme, in the order `theme preview --theme` lists
// them and the order the appearance settings page documents.
func Names() []string {
	return []string{ember.name, emberLight.name, contrast.name, ansiTheme.name, mono.name}
}

// Default is the theme a user who has chosen nothing gets.
const Default = "ember"
