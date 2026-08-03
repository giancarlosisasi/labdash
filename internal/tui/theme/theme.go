// Package theme is labdash's whole visual vocabulary: colour, glyph, word and
// date.
//
// Every screen renders through it, so the app looks native in any terminal,
// stays readable with colour stripped or reduced, and never shifts a column
// because of a wide glyph.
//
// Three decisions shape the API:
//
//   - Tokens are fields on a Theme value, not a lookup by string. A typo is a
//     compile error, and the render path does no allocation per cell.
//   - The colour tier is resolved at construction, never at render time. No
//     call site branches on "what tier am I at", which keeps the branch out of
//     the hottest loop and makes a golden file per tier meaningful.
//   - A Theme is passed explicitly and never stored in a package-level
//     variable. A global theme is untestable in parallel and makes "which theme
//     rendered this golden" unanswerable.
//
// research/17-design-system.md is the source of the vocabulary; this package
// implements it and does not redesign it.
package theme

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/giancarlosisasi/labdash/internal/clock"
)

// A Theme is one (palette, colour tier, icon tier) resolved into the values a
// component renders with.
type Theme struct {
	// Name is the shipped theme this came from.
	Name string
	// Tier is the colour capability every token below is already resolved to.
	Tier Tier
	// Icons is the glyph tier the vocabulary is read at.
	Icons IconTier

	// Surfaces.
	BgBase, BgSurface, BgOverlay, BgSelected color.Color
	BorderFaint, BorderDefault, BorderFocus  color.Color

	// Text.
	TextPrimary, TextSecondary, TextMuted, TextInverted color.Color

	// Meaning.
	AccentPrimary, AccentSecondary            color.Color
	StatusSuccess, StatusWarning, StatusError color.Color
	StatusRunning, StatusPending, StatusNeutral color.Color
	DiffAdded, DiffRemoved                    color.Color

	// SelectionReverse reports that the selected row is drawn with reverse
	// video rather than a background colour. It is true below 256 colours,
	// because reverse video is the only background a sixteen-colour terminal
	// renders reliably across themes.
	SelectionReverse bool

	// Dates is how a timestamp is rendered, against an injected clock.
	Dates DateFormat

	// overrides records which tokens the user replaced. A theme with none is
	// entirely ours; a theme with some is reported on by `theme preview` rather
	// than refused.
	overrides map[Token]bool

	// authored holds the 24-bit value of every token, whatever tier the fields
	// above resolved to.
	//
	// Contrast is measured against these rather than against the resolved
	// values, because a ratio is a property of the design and not of the
	// terminal: at sixteen colours the hues are the user's own and there is
	// nothing of ours to measure, and at monochrome every pair would compute as
	// 1:1 and the whole assertion would pass by being meaningless.
	authored map[Token]color.Color

	// textMin and contrastReason carry the palette's contrast policy, so the
	// test and the preview read one source rather than two.
	textMin        float64
	contrastReason string
}

// Options is what New needs. The zero value builds the default theme for the
// real environment.
type Options struct {
	// Name selects a shipped theme. Empty means ember.
	Name string
	// Env is where NO_COLOR, TERM, COLORTERM and the locale are read. Nil means
	// the real process environment.
	Env Environment
	// Tier forces the colour tier. TierUnset means detect it from Env. NO_COLOR
	// still wins over it, which is what makes THM-06 unconditional rather than
	// a default.
	Tier Tier
	// Icons is the setting. Empty means auto.
	Icons IconSetting
	// NoColor is the --no-color flag. Like NO_COLOR it can only remove colour,
	// never add it.
	NoColor bool
	// Colors replaces individual tokens with the user's own values, in memory.
	// A value below WCAG AA is loaded and reported on, not refused: a personal
	// override on a personal machine is theirs to get wrong, provided the tool
	// says so. THM-04.T2.
	Colors map[Token]string
	// Clock is what a relative timestamp is measured against. Nil means the
	// system clock.
	Clock clock.Clock
	// DatePattern and Timezone control the absolute date form.
	DateFormat  DateStyle
	DatePattern string
	Timezone    string
}

// New resolves a theme. It returns an error only for input the user typed: an
// unknown theme name, an unknown icon setting, an unparseable colour override,
// or a timezone the system does not carry.
func New(opts Options) (Theme, error) {
	env := opts.Env
	if env == nil {
		env = OSEnvironment()
	}

	name := opts.Name
	if name == "" {
		name = Default
	}
	p, ok := palettes[name]
	if !ok {
		return Theme{}, fmt.Errorf(
			"%q is not a theme labdash ships. The themes are %s",
			name, strings.Join(Names(), ", "))
	}

	switch opts.Icons {
	case "", IconsAuto, IconsUnicode, IconsASCII:
	default:
		return Theme{}, fmt.Errorf(
			"%q is not an icon setting. It is one of auto, unicode, ascii", opts.Icons)
	}
	setting := opts.Icons
	if setting == "" {
		setting = IconsAuto
	}

	tier := opts.Tier
	if p.pinnedTier != TierUnset {
		// ansi and mono exist to stay at one tier whatever the terminal says.
		tier = p.pinnedTier
	} else if tier == TierUnset {
		tier = DetectTier(env)
	}

	// NO_COLOR and --no-color are applied last and only ever downward, so no
	// setting, theme or forced tier can turn colour back on. THM-06, ACC-03.
	if opts.NoColor || NoColorSet(env) {
		tier = Monochrome
	}

	tones := make(map[Token]tone, len(p.tones))
	for k, v := range p.tones {
		tones[k] = v
	}

	overrides := map[Token]bool{}
	for token, value := range opts.Colors {
		base, ok := tones[token]
		if !ok {
			return Theme{}, fmt.Errorf(
				"%q is not a theme token. Run `labdash theme preview` to see the set", token)
		}
		if _, err := parseHex(value); err != nil {
			return Theme{}, fmt.Errorf("the value for %s is not recognised: %w", token, err)
		}
		base.hex = value
		tones[token] = base
		overrides[token] = true
	}

	dates, err := newDateFormat(opts)
	if err != nil {
		return Theme{}, err
	}

	authored := make(map[Token]color.Color, len(tones))
	for token, v := range tones {
		c, err := parseHex(v.hex)
		if err != nil {
			return Theme{}, fmt.Errorf("the %s theme's %s is not a colour: %w", p.name, token, err)
		}
		authored[token] = c
	}

	t := Theme{
		Name:             p.name,
		Tier:             tier,
		Icons:            ResolveIcons(setting, env),
		SelectionReverse: tier <= ANSI,
		Dates:            dates,
		overrides:        overrides,
		authored:         authored,
		textMin:          p.textMin,
		contrastReason:   p.contrastReason,
	}

	at := func(token Token) color.Color { return tones[token].resolve(tier) }

	t.BgBase, t.BgSurface = at(BgBase), at(BgSurface)
	t.BgOverlay, t.BgSelected = at(BgOverlay), at(BgSelected)
	t.BorderFaint, t.BorderDefault, t.BorderFocus = at(BorderFaint), at(BorderDefault), at(BorderFocus)
	t.TextPrimary, t.TextSecondary = at(TextPrimary), at(TextSecondary)
	t.TextMuted, t.TextInverted = at(TextMuted), at(TextInverted)
	t.AccentPrimary, t.AccentSecondary = at(AccentPrimary), at(AccentSecondary)
	t.StatusSuccess, t.StatusWarning, t.StatusError = at(StatusSuccess), at(StatusWarning), at(StatusError)
	t.StatusRunning, t.StatusPending, t.StatusNeutral = at(StatusRunning), at(StatusPending), at(StatusNeutral)
	t.DiffAdded, t.DiffRemoved = at(DiffAdded), at(DiffRemoved)

	return t, nil
}

// resolve turns an authored tone into the colour for one tier.
//
// This is the whole of colour-profile degradation, and it happens exactly once
// per theme. Post-processing a rendered frame to strip or downsample colour is
// how a layout drifts between coloured and uncoloured output; building the
// theme without colour keeps "identical column boundaries" true by
// construction.
func (t tone) resolve(tier Tier) color.Color {
	switch tier {
	case TrueColor:
		c, err := parseHex(t.hex)
		if err != nil {
			return lipgloss.NoColor{}
		}
		return c
	case ANSI256:
		c, err := parseHex(t.hex)
		if err != nil {
			return lipgloss.NoColor{}
		}
		return colorprofile.ANSI256.Convert(c)
	case ANSI:
		if t.ansi == noANSI {
			return lipgloss.NoColor{}
		}
		return ansi.BasicColor(t.ansi) //nolint:gosec // the palette's indices are 0–15 by construction
	default:
		return lipgloss.NoColor{}
	}
}

// parseHex reads #RGB or #RRGGBB.
func parseHex(s string) (color.RGBA, error) {
	v := strings.TrimPrefix(s, "#")
	switch len(v) {
	case 3:
		v = string([]byte{v[0], v[0], v[1], v[1], v[2], v[2]})
	case 6:
	default:
		return color.RGBA{}, fmt.Errorf(
			"%q is not a colour. Write it as #RRGGBB, for example #E8A33D", s)
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(strings.ToLower(v), "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.RGBA{}, fmt.Errorf(
			"%q is not a colour. Write it as #RRGGBB, for example #E8A33D", s)
	}
	return color.RGBA{R: r, G: g, B: b, A: 0xff}, nil
}

// Overridden reports whether the user replaced this token.
func (t Theme) Overridden(token Token) bool { return t.overrides[token] }

// Authored returns the 24-bit value in use for a token, override included,
// whatever tier the terminal could give. A reader comparing two terminals needs
// to see what the theme asked for as well as what they got.
func (t Theme) Authored(token Token) string {
	c, ok := t.authored[token]
	if !ok {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
}

// Style returns a style painting text in one role. It is how a component names
// a colour: theme.Style(th.StatusError), never a hex value.
func (t Theme) Style(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

// Glyph returns one status's form for this theme's icon tier.
func (t Theme) Glyph(g Glyph) string { return g.In(t.Icons) }

// Copy folds a string of user-facing prose into this theme's icon tier, so that
// an ellipsis or an em dash in a sentence does not put a byte above 0x7F on an
// ASCII-mode screen.
func (t Theme) Copy(s string) string { return FoldASCII(s, t.Icons) }

// Ellipsis, Borders, SpinnerFrames and ReducedMotionMarker read the icon tier
// off the theme, so a call site never asks which tier it is at.
func (t Theme) Ellipsis() string              { return Ellipsis(t.Icons) }
func (t Theme) Borders() Borders              { return BordersFor(t.Icons) }
func (t Theme) SpinnerFrames() []string       { return SpinnerFrames(t.Icons) }
func (t Theme) ReducedMotionMarker() string   { return ReducedMotionMarker(t.Icons) }

// Truncate cuts to w cells with this theme's ellipsis.
func (t Theme) Truncate(s string, w int) string { return Truncate(s, w, t.Ellipsis()) }

// Pad fits s to exactly w cells, left-aligned.
func (t Theme) Pad(s string, w int) string { return Pad(s, w, t.Ellipsis()) }

// PadLeft fits s to exactly w cells, right-aligned.
func (t Theme) PadLeft(s string, w int) string { return PadLeft(s, w, t.Ellipsis()) }

// A Pair is one measured contrast: a foreground role against a background,
// with the ratio it produced and the ratio it owed.
type Pair struct {
	Foreground Token
	Background Token
	Ratio      float64
	Required   float64
	// Overridden marks a pair involving a colour the user replaced.
	Overridden bool
}

// Meets reports whether the pair cleared what it owed.
func (p Pair) Meets() bool { return p.Ratio >= p.Required }

// String is the line a failing assertion and the preview both print.
func (p Pair) String() string {
	return fmt.Sprintf("%s on %s: %s (needs %s)",
		p.Foreground, p.Background, FormatRatio(p.Ratio), FormatRatio(p.Required))
}

// ContrastPolicy is what a theme owes. Required is zero for a theme whose
// colours come from the terminal, and then Reason says why nothing is asserted.
type ContrastPolicy struct {
	TextRequired float64
	UIRequired   float64
	Reason       string
}

// Policy returns this theme's contrast policy.
func (t Theme) Policy() ContrastPolicy {
	if t.textMin == 0 {
		return ContrastPolicy{Reason: t.contrastReason}
	}
	return ContrastPolicy{TextRequired: t.textMin, UIRequired: RatioUI}
}

// Contrast measures every pair the policy binds: each text and meaning token
// against every surface, each UI token against bg.base, and text.inverted
// against the accents it is drawn on.
//
// It returns an empty slice for a theme whose colours the terminal supplies —
// there is nothing to measure, and Policy().Reason says so.
func (t Theme) Contrast() []Pair {
	policy := t.Policy()
	if policy.TextRequired == 0 {
		return nil
	}

	byToken := t.authored

	var pairs []Pair
	for _, r := range Roles() {
		switch r.Kind {
		case KindText, KindMeaning:
			for _, bg := range Surfaces() {
				pairs = append(pairs, Pair{
					Foreground: r.Token, Background: bg,
					Ratio:    Ratio(byToken[r.Token], byToken[bg]),
					Required: policy.TextRequired,
					Overridden: t.Overridden(r.Token) || t.Overridden(bg),
				})
			}
		case KindUI:
			pairs = append(pairs, Pair{
				Foreground: r.Token, Background: BgBase,
				Ratio:    Ratio(byToken[r.Token], byToken[BgBase]),
				Required: policy.UIRequired,
				Overridden: t.Overridden(r.Token) || t.Overridden(BgBase),
			})
		case KindInverted:
			// Inverted text is drawn on a filled accent, never on a surface.
			for _, bg := range []Token{AccentPrimary, AccentSecondary} {
				pairs = append(pairs, Pair{
					Foreground: r.Token, Background: bg,
					Ratio:    Ratio(byToken[r.Token], byToken[bg]),
					Required: policy.TextRequired,
					Overridden: t.Overridden(r.Token) || t.Overridden(bg),
				})
			}
		}
	}

	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].Ratio < pairs[j].Ratio })
	return pairs
}

// Palette returns the authored 24-bit value of every token, whatever tier the
// theme resolved to.
//
// The preview prints it beside each swatch so that a reader can see what the
// theme asked for as well as what their terminal could give them.
func Palette(name string) (map[Token]string, error) {
	p, ok := palettes[name]
	if !ok {
		return nil, fmt.Errorf("%q is not a theme labdash ships", name)
	}
	out := make(map[Token]string, len(p.tones))
	for k, v := range p.tones {
		out[k] = strings.ToUpper(v.hex)
	}
	return out, nil
}
