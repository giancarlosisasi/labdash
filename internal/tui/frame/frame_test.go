package frame_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/golden"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
	"github.com/giancarlosisasi/labdash/internal/testsupport/termcap"
	"github.com/giancarlosisasi/labdash/internal/tui/frame"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

func TestMain(m *testing.M) { harness.Main(m) }

// SHL-03.T1. Prevents tabs running off the edge with no sign there are more,
// which is the difference between a carousel and a truncation.
func TestSHL03_T1_NineSectionsInEightyColumns(t *testing.T) {
	t.Parallel()

	tabs := make([]frame.Tab, 9)
	for i := range tabs {
		tabs[i] = frame.Tab{Name: fmt.Sprintf("Section %d", i+1), Count: (i + 1) * 7}
	}

	for _, active := range []int{0, 4, 8} {
		th := themeFor(t, termcap.TrueColor)
		row := frame.SectionTabs(th, 76, tabs, active)

		require.Contains(t, row, fmt.Sprintf("Section %d", active+1),
			"the active tab is off screen with tab %d selected", active+1)

		hasLeft := strings.Contains(row, "‹")
		hasRight := strings.Contains(row, "›")
		require.True(t, hasLeft || hasRight,
			"nine sections fit in 76 columns, so the overflow arrows prove nothing")

		golden.Assert(t, fmt.Sprintf("sections.nine.80.active-%d", active+1), row+"\n")
	}
}

// The carousel keeps its geometry whatever is selected. Prevents the tab row
// shifting sideways as you move along it, which makes a tab hard to click and
// harder to trust.
func TestSHL03_TheTabRowIsAlwaysTheSameWidth(t *testing.T) {
	t.Parallel()

	th := themeFor(t, termcap.TrueColor)
	tabs := []frame.Tab{{Name: "One", Count: 1}, {Name: "Two", Count: 22}, {Name: "Three", Count: 333}}

	for _, width := range []int{40, 76, 116, 156} {
		for active := range tabs {
			require.Equal(t, width, theme.Width(frame.SectionTabs(th, width, tabs, active)),
				"width %d, tab %d", width, active)
		}
	}
}

// SHL-07.T1. Prevents a footer that grows and shifts the table above it.
func TestSHL07_T1_ARunningTaskAndAnErrorShareOneLine(t *testing.T) {
	t.Parallel()

	th := themeFor(t, termcap.TrueColor)
	state := frame.FooterState{
		Left:    "Merge requests",
		Spinner: th.SpinnerFrames()[2],
		Task:    "refreshing platform/*",
		Errors:  2,
		Hints: []frame.Hint{
			{Key: "b", Word: "browse", Available: true, Essential: true},
			{Key: "f", Word: "filter", Available: true, Essential: true},
			{Key: "m", Word: "merge", Available: false},
			{Key: "?", Word: "help", Available: true, Essential: true},
			{Key: "q", Word: "quit", Available: true, Essential: true},
		},
	}

	for _, width := range []int{52, 76, 116, 156} {
		row := frame.Footer(th, width, state)

		require.NotContains(t, row, "\n", "the footer grew a second line at %d columns", width)
		require.Equal(t, width, theme.Width(row), "at %d columns", width)
		require.Contains(t, row, state.Spinner,
			"the running task disappeared at %d columns", width)
		require.Contains(t, row, "2", "the queued errors disappeared at %d columns", width)
	}

	// Wide enough to say what is running, and it says so.
	require.Contains(t, frame.Footer(th, 116, state), "refreshing platform/*")

	golden.Assert(t, "footer.task-and-error.116", frame.Footer(th, 116, state)+"\n")
}

// Regression, found in a real terminal on 2026-08-03: at 120 columns the footer
// showed "Cannot send it for review — it i…". Prevents a refusal being cut
// short, which explains nothing and is the exact failure the availability
// predicate exists to avoid.
func TestSHL24_ARefusalIsNeverCutShort(t *testing.T) {
	t.Parallel()

	th := themeFor(t, termcap.TrueColor)
	message := "Cannot send it for review — it is not in this release yet."

	state := frame.FooterState{
		Left:    "Merge requests",
		Message: message,
		Hints: []frame.Hint{
			{Key: "b", Word: "browse", Available: true, Essential: true},
			{Key: "f", Word: "filter", Available: true, Essential: true},
			{Key: "P", Word: "pin", Available: true, Essential: true},
			{Key: "v", Word: "approve", Available: false},
			{Key: "?", Word: "help", Available: true, Essential: true},
			{Key: "q", Word: "quit", Available: true, Essential: true},
		},
	}

	for _, width := range []int{92, 116, 156, 196} {
		row := frame.Footer(th, width, state)

		require.Contains(t, row, message, "the refusal was cut at %d columns", width)
		require.Equal(t, width, theme.Width(row), "at %d columns", width)
		require.Contains(t, row, "quit", "the way out went with it at %d columns", width)
	}

	// Narrower than the message, so it has to give: the hints keep a third of
	// the row and the refusal takes the rest.
	narrow := frame.Footer(th, 76, state)
	require.Equal(t, 76, theme.Width(narrow))
	require.Contains(t, narrow, "quit")

	golden.Assert(t, "footer.refusal.116", frame.Footer(th, 116, state)+"\n")
}

// An unavailable action stays where it is and goes grey. Prevents the footer
// rearranging itself as availability changes, which is the same row moving for
// a reason the user cannot see.
func TestSHL24_TheFooterGreysWhatItCannotRun(t *testing.T) {
	t.Parallel()

	th := themeFor(t, termcap.TrueColor)
	hints := []frame.Hint{
		{Key: "v", Word: "approve", Available: true},
		{Key: "m", Word: "merge", Available: false},
	}

	available := frame.Footer(th, 76, frame.FooterState{Hints: hints})
	hints[0].Available = false
	unavailable := frame.Footer(th, 76, frame.FooterState{Hints: hints})

	require.Equal(t, theme.Width(available), theme.Width(unavailable))
	require.NotEqual(t, available, unavailable, "greying an action changed nothing on screen")
	golden.Assert(t, "footer.unavailable.76", available+"\n"+unavailable+"\n")
}

// The frame is drawn at every colour tier and both icon modes, at identical
// column boundaries. Prevents a wide glyph or an ASCII fallback moving a border.
func TestSHL_TheFrameHasTheSameColumnsInBothIconModes(t *testing.T) {
	t.Parallel()

	termcap.Matrix(t, func(t *testing.T, tier termcap.Tier) {
		var widths []int
		for _, icons := range []theme.IconSetting{theme.IconsUnicode, theme.IconsASCII} {
			th, err := theme.New(theme.Options{Env: tier.Terminal, Icons: icons})
			require.NoError(t, err)

			box := frame.Box{Theme: th, Width: 80}
			lines := []string{
				box.Top(),
				box.Row(frame.ContextBar(th, 76, dashboardTabs(), 0, []string{"offline"})),
				box.SplitRule(40, true),
				box.SplitRow("table", 38, "preview", 34),
				box.SplitRule(40, false),
				box.Bottom(),
			}
			golden.Assert(t, termcap.GoldenName("frame", tier.Name, string(icons)),
				strings.Join(lines, "\n")+"\n")

			for _, line := range lines {
				widths = append(widths, theme.Width(line))
			}
		}

		half := len(widths) / 2
		require.Equal(t, widths[:half], widths[half:],
			"the frame is a different shape in ASCII mode")
	})
}

func dashboardTabs() []frame.Tab {
	return []frame.Tab{
		{Name: "Merge requests", Count: -1},
		{Name: "Pipelines", Count: -1},
		{Name: "To-Dos", Count: -1},
		{Name: "Issues", Count: -1},
	}
}

func themeFor(t *testing.T, term termcap.Terminal) theme.Theme {
	t.Helper()

	th, err := theme.New(theme.Options{Env: term, Icons: theme.IconsUnicode})
	require.NoError(t, err)
	return th
}
