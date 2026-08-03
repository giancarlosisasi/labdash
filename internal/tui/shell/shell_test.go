package shell_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
	"github.com/giancarlosisasi/labdash/internal/tui/shell"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

func TestMain(m *testing.M) { harness.Main(m) }

// SHL-02.T1. Prevents switching views resetting your place, which turns Tab
// from a way to look at something else into a way to lose what you were doing.
func TestSHL02_T1_EachViewKeepsItsOwnPlace(t *testing.T) {
	t.Parallel()

	m := newShell(t, 120, 32)

	press(t, m, "l")   // merge requests: second section
	press(t, m, "tab") // pipelines
	press(t, m, "l")   //
	press(t, m, "l")   // pipelines: third section
	press(t, m, "tab", "tab", "tab")

	require.Equal(t, "Merge requests", m.Active().Title())
	require.Equal(t, 1, m.Active().Section())

	press(t, m, "tab")
	require.Equal(t, "Pipelines", m.Active().Title())
	require.Equal(t, 2, m.Active().Section())
}

// KEY-01 and KEY-02 as behaviour rather than as a table: every vim key does its
// job, and every arrow does the same job.
func TestKEY01_T1_TheVimKeysAndTheArrowsAgree(t *testing.T) {
	t.Parallel()

	pairs := [][2]string{{"l", "right"}, {"h", "left"}}

	for _, pair := range pairs {
		withVim, withArrow := newShell(t, 120, 32), newShell(t, 120, 32)
		press(t, withVim, pair[0])
		press(t, withArrow, pair[1])

		require.Equal(t, withVim.Active().Section(), withArrow.Active().Section(),
			"%q and %q did different things", pair[0], pair[1])
	}

	// Tab cycles the views and Shift+Tab reverses it, which is the only
	// non-vim way round the four.
	m := newShell(t, 120, 32)
	press(t, m, "tab")
	require.Equal(t, "Pipelines", m.Active().Title())
	press(t, m, "shift+tab")
	require.Equal(t, "Merge requests", m.Active().Title())
}

// SHL-09.T2, and gh-dash #578 with it: a crash in the help overlay.
func TestSHL09_T2_AnyKeyClosesTheOverlay(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"esc", "q", "?", "a", "ctrl+r", "enter", "tab"} {
		m := newShell(t, 120, 32)
		press(t, m, "?")
		require.Contains(t, m.View().Content, "Help", "the overlay did not open")

		press(t, m, key)
		require.NotContains(t, m.View().Content, "Help",
			"%q left the overlay open", key)
	}
}

// REG-05. Prevents a panic on input nobody thought to try: a paste, a dead key,
// an emoji, a CJK character, a lone combining mark.
func TestREG05_TheOverlayAndTheShellSurviveArbitraryInput(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"a", "Z", "0", " ", "\t", "é", "日", "🦝", "👩‍👩‍👧‍👦", "́", "​",
		"\x00", "\x1b", "ctrl+a", "alt+x", "f13", strings.Repeat("x", 4096),
	}

	for _, in := range inputs {
		m := newShell(t, 120, 32)
		require.NotPanics(t, func() {
			m.Update(tea.KeyPressMsg{Text: in})
			m.View()
			m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
			m.Update(tea.KeyPressMsg{Text: in})
			m.View()
		}, "input %q", in)
	}
}

// A fixed keymap cannot afford a silent keypress. Prevents a key that is
// published, pressed, and answers nothing.
func TestSHL24_AnUnavailableKeyAnswersInTheFooter(t *testing.T) {
	t.Parallel()

	m := newShell(t, 120, 32)
	press(t, m, "m")

	content := m.View().Content
	require.Contains(t, content, "Cannot merge",
		"pressing a key bound to an unbuilt action said nothing")
}

// Esc goes back one level and never quits; q quits, and not from an overlay.
func TestSHL_EscGoesBackAndQuits(t *testing.T) {
	t.Parallel()

	m := newShell(t, 120, 32)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd, "Esc on a dashboard tried to do something")

	press(t, m, "?")
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	require.Nil(t, cmd, "q quit from inside an overlay")
	require.NotContains(t, m.View().Content, "Help")

	_, cmd = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	require.NotNil(t, cmd, "q did not quit from a dashboard")
}

// SHL-14.T1 at the level a user sees: the same terminal size renders the same
// frame, whatever it went through in between.
func TestSHL14_T1_AResizeRoundTripRedrawsTheSameFrame(t *testing.T) {
	t.Parallel()

	m := newShell(t, 200, 50)
	before := m.View().Content

	for _, w := range []int{160, 120, 90, 56, 40, 90, 160} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 50})
		m.View()
	}
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})

	require.Equal(t, before, m.View().Content)
}

// Prevents the frame losing or gaining a row, which moves the footer and with
// it every row above.
func TestSHL_EveryFrameIsExactlyTheTerminalSize(t *testing.T) {
	t.Parallel()

	for _, size := range [][2]int{{56, 12}, {80, 24}, {120, 32}, {160, 40}, {200, 50}} {
		m := newShell(t, size[0], size[1])
		lines := strings.Split(m.View().Content, "\n")

		require.Len(t, lines, size[1], "at %dx%d", size[0], size[1])
		for i, line := range lines {
			require.Equal(t, size[0], theme.Width(line),
				"line %d is %d cells at %dx%d", i+1, theme.Width(line), size[0], size[1])
		}
	}
}

// Suspending and resuming restores the screen exactly. The shell holds no
// state that a suspend can disturb, and this is what keeps it that way.
func TestSHL_SuspendAndResumeRestoreTheScreen(t *testing.T) {
	t.Parallel()

	m := newShell(t, 120, 32)
	press(t, m, "tab", "l")
	before := m.View().Content

	m.Update(tea.SuspendMsg{})
	m.Update(tea.ResumeMsg{})

	require.Equal(t, before, m.View().Content)
	require.Equal(t, "Pipelines", m.Active().Title(), "the resumed shell forgot where it was")
}

// SHL-15.T2 as a user meets it: the notice names both widths, so nobody has to
// guess how far to drag.
func TestSHL15_T2_TheTooNarrowNoticeNamesBothWidths(t *testing.T) {
	t.Parallel()

	content := newShell(t, 40, 20).View().Content
	require.Contains(t, content, "40 columns")
	require.Contains(t, content, "56")
	require.Contains(t, content, "quit")
}

func lineWidths(content string) []int {
	var out []int
	for _, line := range strings.Split(content, "\n") {
		out = append(out, theme.Width(line))
	}
	return out
}

func newShell(t *testing.T, width, height int) *shell.Model {
	t.Helper()

	th, err := theme.New(theme.Options{Tier: theme.Monochrome, Icons: theme.IconsUnicode})
	require.NoError(t, err)

	return shell.New(shell.Options{
		Theme: th, Scope: action.ScopeWrite, Width: width, Height: height,
	})
}

// press sends keystrokes the way a terminal reports them, resolving each
// through the keymap first so a test can never exercise a key the product does
// not bind.
func press(t *testing.T, m *shell.Model, keys ...string) {
	t.Helper()

	for _, k := range keys {
		_, ok := keymap.Lookup(m.Active().Screen(), k)
		require.True(t, ok || k == "?", "%q is not bound on %s", k, m.Active().Screen())
		m.Update(keyPress(k))
	}
}

func keyPress(s string) tea.KeyPressMsg {
	switch s {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	if rest, chord := strings.CutPrefix(s, "ctrl+"); chord {
		return tea.KeyPressMsg{Code: rune(rest[0]), Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}
