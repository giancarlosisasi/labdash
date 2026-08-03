package theme

import "image/color"

// A Token names one semantic role.
//
// A component names a role and never a hex value, so a palette change reaches
// every screen at once and theming is not a lie. THM-01.T1 fails the build on a
// colour literal anywhere outside this package.
//
// The set is fixed. It is the whole vocabulary in
// research/17-design-system.md §2.2, and a screen that wants a colour outside
// it wants a colour the theme cannot honour at sixteen colours.
type Token string

const (
	BgBase     Token = "bg.base"
	BgSurface  Token = "bg.surface"
	BgOverlay  Token = "bg.overlay"
	BgSelected Token = "bg.selected"

	BorderFaint   Token = "border.faint"
	BorderDefault Token = "border.default"
	BorderFocus   Token = "border.focus"

	TextPrimary   Token = "text.primary"
	TextSecondary Token = "text.secondary"
	TextMuted     Token = "text.muted"
	TextInverted  Token = "text.inverted"

	AccentPrimary   Token = "accent.primary"
	AccentSecondary Token = "accent.secondary"

	StatusSuccess Token = "status.success"
	StatusWarning Token = "status.warning"
	StatusError   Token = "status.error"
	StatusRunning Token = "status.running"
	StatusPending Token = "status.pending"
	StatusNeutral Token = "status.neutral"

	DiffAdded   Token = "diff.added"
	DiffRemoved Token = "diff.removed"
)

// A Kind groups tokens by what the contrast rule owes them.
type Kind uint8

const (
	// KindSurface is a background. Text is measured against it; it is not
	// measured against anything itself.
	KindSurface Kind = iota
	// KindText is read closely and owes 4.5:1 against every surface.
	KindText
	// KindMeaning is an accent or a status colour. It is drawn as text — a
	// glyph, a word, a count — so it owes the same 4.5:1.
	KindMeaning
	// KindUI identifies a component or its state. It owes 3:1.
	KindUI
	// KindDecoration separates or rules. It owes nothing, and each entry
	// carries the reason it is here.
	KindDecoration
	// KindInverted is drawn on a filled accent rather than on a surface, so it
	// is measured against the accents instead.
	KindInverted
)

// A Role is a token paired with the field that holds it.
//
// The struct fields are the API — naming one is a compile-time check, where a
// map lookup by string would move a typo to render time. Role exists so that
// the contrast test and `theme preview` can walk the whole set without turning
// the call-site API into a map.
type Role struct {
	Token Token
	Kind  Kind
	// Use is the phrase the design system and the preview both print.
	Use string
	// Of reads the token off a theme.
	Of func(Theme) color.Color
	// DecorationReason is why a KindDecoration token is exempt from the 3:1
	// rule. It is required for that kind and empty for every other, and the
	// contrast test asserts both — an exemption with no reason is how a rule
	// stops applying to anything.
	DecorationReason string
}

// Roles is every token, in the order the design system lists them and the
// order `theme preview` prints them.
func Roles() []Role {
	return []Role{
		{BgBase, KindSurface, "The frame", func(t Theme) color.Color { return t.BgBase }, ""},
		{BgSurface, KindSurface, "Header, footer, filter bar", func(t Theme) color.Color { return t.BgSurface }, ""},
		{BgOverlay, KindSurface, "Overlays and modals", func(t Theme) color.Color { return t.BgOverlay }, ""},
		{BgSelected, KindSurface, "The selected row", func(t Theme) color.Color { return t.BgSelected }, ""},

		{BorderFaint, KindDecoration, "Row separators, inner rules",
			func(t Theme) color.Color { return t.BorderFaint },
			"A row separator carries no state. At 3:1 it would draw a grid louder than the rows."},
		{BorderDefault, KindDecoration, "Pane and overlay borders",
			func(t Theme) color.Color { return t.BorderDefault },
			"An unfocused pane's edge carries no state either; border.focus is what says which pane has focus, and it is held to 3:1."},
		{BorderFocus, KindUI, "The focused pane", func(t Theme) color.Color { return t.BorderFocus }, ""},

		{TextPrimary, KindText, "Titles and values", func(t Theme) color.Color { return t.TextPrimary }, ""},
		{TextSecondary, KindText, "Authors, projects, metadata", func(t Theme) color.Color { return t.TextSecondary }, ""},
		{TextMuted, KindText, "Timestamps, hints, disabled", func(t Theme) color.Color { return t.TextMuted }, ""},
		{TextInverted, KindInverted, "Text on a filled accent", func(t Theme) color.Color { return t.TextInverted }, ""},

		{AccentPrimary, KindMeaning, "Brand, focus, active tab", func(t Theme) color.Color { return t.AccentPrimary }, ""},
		{AccentSecondary, KindMeaning, "Links and references", func(t Theme) color.Color { return t.AccentSecondary }, ""},

		{StatusSuccess, KindMeaning, "passed, merged, approved", func(t Theme) color.Color { return t.StatusSuccess }, ""},
		{StatusWarning, KindMeaning, "manual, stale, allow-failure", func(t Theme) color.Color { return t.StatusWarning }, ""},
		{StatusError, KindMeaning, "failed, conflicts, blocked", func(t Theme) color.Color { return t.StatusError }, ""},
		{StatusRunning, KindMeaning, "running, in progress", func(t Theme) color.Color { return t.StatusRunning }, ""},
		{StatusPending, KindMeaning, "pending, queued, scheduled", func(t Theme) color.Color { return t.StatusPending }, ""},
		{StatusNeutral, KindMeaning, "skipped, canceled, created", func(t Theme) color.Color { return t.StatusNeutral }, ""},

		{DiffAdded, KindMeaning, "+412", func(t Theme) color.Color { return t.DiffAdded }, ""},
		{DiffRemoved, KindMeaning, "-38", func(t Theme) color.Color { return t.DiffRemoved }, ""},
	}
}

// Surfaces is every background a text token may be drawn on. The contrast rule
// binds text against all of them, not only against bg.base: the pair that
// fails in practice is a timestamp on the selected row, which is the most
// scanned cell in the product.
func Surfaces() []Token { return []Token{BgBase, BgSurface, BgOverlay, BgSelected} }

// Color reads one token off the theme by name. Components use the struct
// fields; this is for the preview, the contrast test and anything else that
// walks the whole set.
func (t Theme) Color(token Token) color.Color {
	for _, r := range Roles() {
		if r.Token == token {
			return r.Of(t)
		}
	}
	return nil
}
