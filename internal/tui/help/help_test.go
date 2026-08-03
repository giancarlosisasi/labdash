package help_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/testsupport/golden"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
	"github.com/giancarlosisasi/labdash/internal/testsupport/termcap"
	"github.com/giancarlosisasi/labdash/internal/tui/help"
	"github.com/giancarlosisasi/labdash/internal/tui/layout"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

func TestMain(m *testing.M) { harness.Main(m) }

// SHL-09.T1. Prevents help text drifting from the actual keys: every line in
// the overlay is a row of the keymap, and there is no second source for it.
func TestSHL09_T1_TheOverlayIsGeneratedFromTheKeymap(t *testing.T) {
	t.Parallel()

	th := themeFor(t, theme.IconsUnicode)
	l := layout.Compute(120, 32)

	for _, screen := range action.Screens() {
		ctx := action.Context{Screen: screen, Scope: action.ScopeWrite}
		rendered := strings.Join(help.Render(th, l, ctx), "\n")

		listed := help.Bindings(ctx)
		require.NotEmpty(t, listed, "%s offers nothing at all", screen)

		for _, e := range listed {
			require.Contains(t, rendered, e.Help,
				"%s is listed for %s and its help text is not on screen", e.ID, screen)
		}

		for _, e := range keymap.For(screen) {
			if e.Implemented {
				continue
			}
			require.NotContains(t, rendered, e.Help,
				"%s is not built and the overlay offers it anyway", e.ID)
		}
	}
}

// The overlay groups by tier, in learning order. Prevents an alphabetical list,
// which is how a cheatsheet stops being learnable.
func TestSHL09_TheOverlayGroupsByTier(t *testing.T) {
	t.Parallel()

	th := themeFor(t, theme.IconsUnicode)
	rendered := strings.Join(
		help.Render(th, layout.Compute(120, 32),
			action.Context{Screen: action.MergeRequests, Scope: action.ScopeWrite}), "\n")

	var seen []int
	for _, tier := range keymap.Tiers() {
		if i := strings.Index(rendered, strings.ToUpper(tier.Title())); i >= 0 {
			seen = append(seen, i)
		}
	}
	require.NotEmpty(t, seen)
	require.IsIncreasing(t, seen, "the tier headings are out of learning order")
}

// The overlay is bounded and centred whatever the terminal does, and every line
// is exactly the same width. Prevents a ragged box, which is what an overlay
// built by concatenation looks like.
func TestSHL09_TheOverlayIsBoundedAndSquare(t *testing.T) {
	t.Parallel()

	th := themeFor(t, theme.IconsUnicode)
	ctx := action.Context{Screen: action.MergeRequests, Scope: action.ScopeWrite}

	for _, width := range []int{60, 80, 120, 200} {
		l := layout.Compute(width, 32)
		for _, line := range help.Render(th, l, ctx) {
			require.Equal(t, l.OverlayWidth, theme.Width(line),
				"at %d columns: %q", width, line)
		}
	}
}

// The overlay is drawn at every colour tier and both icon modes, and the two
// icon modes have identical column boundaries.
func TestSHL09_T1_Golden(t *testing.T) {
	t.Parallel()

	ctx := action.Context{Screen: action.MergeRequests, Scope: action.ScopeWrite}

	termcap.Matrix(t, func(t *testing.T, tier termcap.Tier) {
		var widths [2][]int
		for i, icons := range []theme.IconSetting{theme.IconsUnicode, theme.IconsASCII} {
			th, err := theme.New(theme.Options{Env: tier.Terminal, Icons: icons})
			require.NoError(t, err)

			lines := help.Render(th, layout.Compute(120, 32), ctx)
			golden.Assert(t, termcap.GoldenName("help", tier.Name, string(icons)),
				strings.Join(lines, "\n")+"\n")

			for _, line := range lines {
				widths[i] = append(widths[i], theme.Width(line))
			}
		}
		require.Equal(t, widths[0], widths[1], "the overlay is a different shape in ASCII mode")
	})
}

// Every screen carries the universal set, so no view can open a help overlay
// with nothing in it. Prevents a view quietly losing the keys it inherits.
func TestSHL09_EveryScreenOffersTheUniversalKeys(t *testing.T) {
	t.Parallel()

	th := themeFor(t, theme.IconsUnicode)

	for _, screen := range action.Screens() {
		rendered := strings.Join(
			help.Render(th, layout.Compute(120, 32), action.Context{Screen: screen}), "\n")

		for _, id := range []keymap.Action{keymap.Help, keymap.Quit, keymap.Back, keymap.Down} {
			e, ok := keymap.Find(id)
			require.True(t, ok)
			require.Contains(t, rendered, e.Help, "%s does not offer %s", screen, id)
		}
		require.Contains(t, rendered, "close")
	}
}

func themeFor(t *testing.T, icons theme.IconSetting) theme.Theme {
	t.Helper()

	th, err := theme.New(theme.Options{Env: termcap.TrueColor, Icons: icons})
	require.NoError(t, err)
	return th
}
