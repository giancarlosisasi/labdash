package theme_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/termcap"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// Prevents: a terminal being read as less capable than it is, which costs the
// palette, or as more, which costs legibility. The table is
// research/17-design-system.md §2.1.
func TestTierDetection(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		env  theme.Environment
		want theme.Tier
	}{
		{"COLORTERM truecolor", termcap.TrueColor, theme.TrueColor},
		{"COLORTERM 24bit", termcap.ANSI256.With("COLORTERM", "24bit"), theme.TrueColor},
		{"TERM 256color", termcap.ANSI256, theme.ANSI256},
		{"TERM direct", termcap.ANSI16.With("TERM", "xterm-direct"), theme.TrueColor},
		{"plain TERM", termcap.ANSI16, theme.ANSI},
		{"TERM dumb", termcap.Dumb, theme.Monochrome},
		{"NO_COLOR", termcap.Monochrome, theme.Monochrome},
		{"Windows Terminal, no TERM", termcap.Terminal{
			Env: map[string]string{"WT_SESSION": "9f2b"},
		}, theme.TrueColor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, theme.DetectTier(tc.env))
		})
	}
}

// THM-06.T1 / ACC-03.T1. Prevents: ignoring a standard that exists precisely
// for accessibility. NO_COLOR is honoured whatever its value, and no setting,
// theme or forced tier can turn colour back on.
func TestTHM06_T1_NoColorIsUnconditional(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"1", "0", "true", "false", "no", "anything"} {
		t.Run("NO_COLOR="+value, func(t *testing.T) {
			t.Parallel()

			env := termcap.TrueColor.With("NO_COLOR", value)

			// Even asked outright for true colour, and for a theme that has
			// one, the tier is monochrome.
			th := build(t, theme.Options{Name: "ember", Env: env, Tier: theme.TrueColor})
			require.Equal(t, theme.Monochrome, th.Tier,
				"a forced tier overrode NO_COLOR")
			require.Empty(t, sgrColours(render(th)),
				"colour reached the output under NO_COLOR")
		})
	}
}

// Prevents: NO_COLOR= (empty) disabling colour. The standard is "present and
// not an empty string", and an empty value is how a shell exports a variable it
// does not mean to set.
func TestAnEmptyNoColorIsNotSet(t *testing.T) {
	t.Parallel()

	th := build(t, theme.Options{Env: termcap.TrueColor.With("NO_COLOR", "")})
	require.Equal(t, theme.TrueColor, th.Tier)
}

// ACC-03.T1, the flag half. Prevents: --no-color being advisory.
func TestTHM06_T1_TheNoColorFlagAlsoWins(t *testing.T) {
	t.Parallel()

	th := build(t, theme.Options{
		Name: "ember", Env: termcap.TrueColor, Tier: theme.TrueColor, NoColor: true,
	})
	require.Equal(t, theme.Monochrome, th.Tier)
	require.Empty(t, sgrColours(render(th)))
}

// Prevents: the mono and ansi themes drifting to whatever the terminal
// reports. Each exists to stay at one tier, which is what makes them useful for
// a screenshot and for a beloved colour scheme respectively.
func TestPinnedThemesIgnoreTheTerminal(t *testing.T) {
	t.Parallel()

	require.Equal(t, theme.Monochrome,
		build(t, theme.Options{Name: "mono", Env: termcap.TrueColor}).Tier)
	require.Equal(t, theme.ANSI,
		build(t, theme.Options{Name: "ansi", Env: termcap.TrueColor}).Tier)
}

// Prevents: the sixteen-colour tier painting a background. A background colour
// at that tier fights whatever theme the user chose; reverse video is the one
// background every terminal renders reliably.
func TestSelectionIsReverseVideoBelow256Colours(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		tier theme.Tier
		want bool
	}{
		{theme.Monochrome, true},
		{theme.ANSI, true},
		{theme.ANSI256, false},
		{theme.TrueColor, false},
	} {
		th := build(t, theme.Options{Env: termcap.ANSI256, Tier: tc.tier})
		require.Equal(t, tc.want, th.SelectionReverse, "at tier %s", tc.tier)
	}
}

// Prevents: a tier resolving colours it cannot render. At sixteen colours the
// palette's declared ANSI index is used rather than a nearest match, because
// "closest to amber" is a judgement and the user's terminal supplies the hue.
func TestEachTierEmitsWhatItCanRender(t *testing.T) {
	t.Parallel()

	trueColour := build(t, theme.Options{Env: termcap.TrueColor, Tier: theme.TrueColor})
	require.Contains(t, render(trueColour), "38;2;",
		"true colour did not emit a 24-bit sequence")

	indexed := build(t, theme.Options{Env: termcap.ANSI256, Tier: theme.ANSI256})
	require.Contains(t, render(indexed), "38;5;",
		"the 256-colour tier did not emit an indexed sequence")

	basic := build(t, theme.Options{Env: termcap.ANSI16, Tier: theme.ANSI})
	out := render(basic)
	require.NotContains(t, out, "38;2;")
	require.NotContains(t, out, "38;5;")
	require.NotEmpty(t, sgrColours(out), "the sixteen-colour tier emitted no colour at all")
}

// Prevents: an unknown theme or icon setting failing with a Go error rather
// than with the list of what is accepted.
func TestUnknownNamesAreExplained(t *testing.T) {
	t.Parallel()

	_, err := theme.New(theme.Options{Name: "solarized", Env: termcap.TrueColor})
	require.ErrorContains(t, err, "ember")
	require.ErrorContains(t, err, "contrast")

	_, err = theme.New(theme.Options{Icons: "nerdfont", Env: termcap.TrueColor})
	require.ErrorContains(t, err, "auto, unicode, ascii")
}

// render paints one word in every status role, which is the smallest string
// that carries every colour a screen would.
func render(th theme.Theme) string {
	var b strings.Builder
	for _, r := range theme.Roles() {
		b.WriteString(th.Style(r.Of(th)).Render(string(r.Token)))
	}
	return b.String()
}

// sgrColours returns every SGR colour parameter in s. Reverse, bold and faint
// are not colours and do not appear here: NO_COLOR removes colour, not text
// decoration.
func sgrColours(s string) []string {
	var out []string
	for _, seq := range strings.Split(s, "\x1b[") {
		end := strings.IndexByte(seq, 'm')
		if end < 0 {
			continue
		}
		for _, param := range strings.Split(seq[:end], ";") {
			switch {
			case param == "38", param == "48", param == "39", param == "49":
				out = append(out, param)
			case len(param) == 2 && (param[0] == '3' || param[0] == '4') &&
				param[1] >= '0' && param[1] <= '7':
				out = append(out, param)
			}
		}
	}
	return out
}
