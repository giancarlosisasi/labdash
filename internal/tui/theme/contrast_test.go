package theme_test

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/termcap"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// THM-02.T1 / ACC-02.T1. Prevents: shipping a theme that is unreadable for
// some users. Every text and meaning token is measured against every surface,
// not only against bg.base — the pair that fails in practice is a timestamp on
// the selected row, which is the most scanned cell in the product.
func TestTHM02_T1_EveryShippedThemeMeetsItsContrast(t *testing.T) {
	t.Parallel()

	for _, name := range theme.Names() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			th := build(t, theme.Options{Name: name, Tier: theme.TrueColor})
			policy := th.Policy()

			if policy.TextRequired == 0 {
				// A theme whose colours come from the terminal cannot have its
				// contrast computed, and it may not pass by omission: it has to
				// say why, in a sentence a reader can disagree with.
				require.NotEmpty(t, policy.Reason,
					"%s asserts no contrast and gives no reason", name)
				require.Empty(t, th.Contrast())
				return
			}

			pairs := th.Contrast()
			require.NotEmpty(t, pairs, "%s claims a contrast policy but measures nothing", name)

			for _, p := range pairs {
				require.True(t, p.Meets(), "%s: %s", name, p)
			}
		})
	}
}

// ACC-05.T1. Prevents: a "high contrast" theme that is merely different. The
// contrast theme owes 7:1 for body text, and it is re-derived rather than
// ember with the saturation turned up.
func TestACC05_T1_TheContrastThemeIsAAA(t *testing.T) {
	t.Parallel()

	th := build(t, theme.Options{Name: "contrast", Tier: theme.TrueColor})
	require.Equal(t, theme.RatioAAAText, th.Policy().TextRequired)

	for _, p := range th.Contrast() {
		if p.Required == theme.RatioAAAText {
			require.GreaterOrEqual(t, p.Ratio, theme.RatioAAAText, p.String())
		}
	}

	ember := build(t, theme.Options{Name: "ember", Tier: theme.TrueColor})
	require.NotEqual(t, ember.BgBase, th.BgBase,
		"contrast reuses ember's background, so it is a tint rather than a re-derivation")
	require.NotEqual(t, ember.StatusError, th.StatusError)
}

// Prevents: a theme passing the contrast assertion because a token is missing
// from its palette and resolved to nothing. Every shipped theme defines every
// token in the set.
func TestEveryThemeDefinesEveryToken(t *testing.T) {
	t.Parallel()

	for _, name := range theme.Names() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			values, err := theme.Palette(name)
			require.NoError(t, err)

			for _, r := range theme.Roles() {
				require.Contains(t, values, r.Token,
					"%s does not define %s", name, r.Token)
				require.NotEmpty(t, values[r.Token])
			}
		})
	}
}

// Prevents: a token quietly dropping out of the contrast rule. Every role is
// either measured or exempt, and every exemption states its reason — an
// exemption with no reason is how a rule stops applying to anything.
func TestEveryExemptionCarriesItsReason(t *testing.T) {
	t.Parallel()

	exempt := map[theme.Token]bool{}
	for _, r := range theme.Roles() {
		if r.Kind == theme.KindDecoration {
			require.NotEmpty(t, r.DecorationReason,
				"%s is exempt from the contrast rule and does not say why", r.Token)
			exempt[r.Token] = true
			continue
		}
		require.Empty(t, r.DecorationReason,
			"%s carries an exemption reason but is not exempt", r.Token)
	}

	// The exempt set is asserted whole, so a new border cannot join it without
	// this test being edited and the reason being read.
	require.Equal(t,
		map[theme.Token]bool{theme.BorderFaint: true, theme.BorderDefault: true},
		exempt)
}

// THM-04.T2. Prevents: refusing a user's own choice on their own machine. An
// override below AA loads, and the tool reports the ratio it produced instead
// of rejecting it. ACC-02.T1 binds the shipped themes; a personal override is
// theirs to get wrong, provided they are told.
func TestTHM04_T2_AnOverrideBelowAALoadsAndIsReported(t *testing.T) {
	t.Parallel()

	th := build(t, theme.Options{
		Name: "ember", Tier: theme.TrueColor,
		Colors: map[theme.Token]string{theme.TextMuted: "#4A4E58"},
	})

	require.True(t, th.Overridden(theme.TextMuted))
	require.False(t, th.Overridden(theme.TextPrimary))

	var reported bool
	for _, p := range th.Contrast() {
		if p.Foreground != theme.TextMuted {
			continue
		}
		require.True(t, p.Overridden, "an overridden pair was not marked as one")
		if !p.Meets() {
			reported = true
			require.Contains(t, p.String(), "needs 4.50:1")
		}
	}
	require.True(t, reported,
		"an override well below AA produced no failing pair to report")
}

// Prevents: an unparseable override bricking startup with a Go error. A value
// the user typed gets a message that says what to write instead.
func TestAnUnrecognisedOverrideIsNamed(t *testing.T) {
	t.Parallel()

	_, err := theme.New(theme.Options{
		Env:    termcap.TrueColor,
		Colors: map[theme.Token]string{theme.TextMuted: "bleu"},
	})
	require.ErrorContains(t, err, "text.muted")
	require.ErrorContains(t, err, "#RRGGBB")

	_, err = theme.New(theme.Options{
		Env:    termcap.TrueColor,
		Colors: map[theme.Token]string{"text.chartreuse": "#AABBCC"},
	})
	require.ErrorContains(t, err, "not a theme token")
}

// Prevents: the contrast arithmetic drifting. These three are fixed points of
// WCAG 2.2 that any implementation must reproduce exactly.
func TestRatioMatchesWCAG(t *testing.T) {
	t.Parallel()

	black := color.RGBA{A: 0xff}
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	grey := color.RGBA{R: 0x77, G: 0x77, B: 0x77, A: 0xff}

	require.InDelta(t, 21.0, theme.Ratio(black, white), 0.001)
	require.InDelta(t, 1.0, theme.Ratio(grey, grey), 0.001)
	require.InDelta(t, theme.Ratio(black, white), theme.Ratio(white, black), 0.001,
		"the ratio is not symmetric")
	require.Equal(t, "21.00:1", theme.FormatRatio(theme.Ratio(black, white)))
}

func build(t *testing.T, opts theme.Options) theme.Theme {
	t.Helper()
	if opts.Env == nil {
		opts.Env = termcap.TrueColor
	}
	th, err := theme.New(opts)
	require.NoError(t, err)
	return th
}
