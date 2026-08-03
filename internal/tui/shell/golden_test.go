package shell_test

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/testsupport/golden"
	"github.com/giancarlosisasi/labdash/internal/testsupport/termcap"
	"github.com/giancarlosisasi/labdash/internal/tui/shell"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// breakpoints is one terminal per width band, at the height the design draws
// each of them at.
var breakpoints = []struct {
	name          string
	width, height int
}{
	{"xs", 40, 20},
	{"s", 70, 24},
	{"m", 100, 28},
	{"l", 120, 32},
	{"xl", 180, 44},
}

// The dashboard at every width band. A diff here is a screen changing, and it
// is read like code.
func TestGoldenDashboardAtEveryBreakpoint(t *testing.T) {
	t.Parallel()

	for _, bp := range breakpoints {
		th, err := theme.New(theme.Options{Env: termcap.TrueColor, Icons: theme.IconsUnicode})
		require.NoError(t, err)

		m := shell.New(shell.Options{
			Theme: th, Scope: action.ScopeWrite, Width: bp.width, Height: bp.height,
		})
		golden.Assert(t, "dashboard."+bp.name, m.View().Content+"\n")
	}
}

// The three states this change can put a dashboard into, at the reference size:
// empty, the help overlay open, and a key that answers with a refusal.
//
// A populated dashboard and a section error belong to the change that brings
// rows; there is nothing here that could fill either.
func TestGoldenDashboardStates(t *testing.T) {
	t.Parallel()

	states := map[string]func(*shell.Model){
		"empty":    func(*shell.Model) {},
		"help":     func(m *shell.Model) { m.Update(tea.KeyPressMsg{Code: '?', Text: "?"}) },
		"refusal":  func(m *shell.Model) { m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"}) },
		"no-token": func(*shell.Model) {},
	}

	for name, drive := range states {
		th, err := theme.New(theme.Options{Env: termcap.TrueColor, Icons: theme.IconsUnicode})
		require.NoError(t, err)

		scope := action.ScopeWrite
		if name == "no-token" {
			scope = action.ScopeNone
		}

		m := shell.New(shell.Options{Theme: th, Scope: scope, Width: 120, Height: 32})
		drive(m)
		golden.Assert(t, "dashboard.state."+name, m.View().Content+"\n")
	}
}

// The reference screen at every colour tier and both icon modes, with a
// no-colour twin for each. Prevents a colour role or a glyph fallback changing
// without anybody seeing it.
func TestGoldenDashboardAcrossTerminals(t *testing.T) {
	t.Parallel()

	termcap.Matrix(t, func(t *testing.T, tier termcap.Tier) {
		for _, icons := range []theme.IconSetting{theme.IconsUnicode, theme.IconsASCII} {
			th, err := theme.New(theme.Options{Env: tier.Terminal, Icons: icons})
			require.NoError(t, err)

			m := shell.New(shell.Options{
				Theme: th, Scope: action.ScopeWrite, Width: 120, Height: 32,
			})
			golden.Assert(t, termcap.GoldenName("dashboard", tier.Name, string(icons)),
				m.View().Content+"\n")
		}
	})
}

// ACC-04 at the screen level: an ASCII-mode render carries no byte above 0x7F,
// and it draws its columns in exactly the same places.
func TestACC04_ASCIIModeIsPlainAndTheSameShape(t *testing.T) {
	t.Parallel()

	for _, bp := range breakpoints {
		var widths [2][]int

		for i, icons := range []theme.IconSetting{theme.IconsUnicode, theme.IconsASCII} {
			th, err := theme.New(theme.Options{Env: termcap.TrueColor, Icons: icons})
			require.NoError(t, err)

			m := shell.New(shell.Options{
				Theme: th, Scope: action.ScopeWrite, Width: bp.width, Height: bp.height,
			})
			content := m.View().Content

			if icons == theme.IconsASCII {
				for _, r := range content {
					require.Less(t, r, rune(0x80),
						"%s carries %q in ASCII mode", bp.name, r)
				}
			}
			widths[i] = lineWidths(content)
		}

		require.Equal(t, widths[0], widths[1],
			fmt.Sprintf("the %s screen is a different shape in ASCII mode", bp.name))
	}
}
