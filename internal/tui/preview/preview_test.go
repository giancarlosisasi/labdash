package preview_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/golden"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
	"github.com/giancarlosisasi/labdash/internal/testsupport/termcap"
	"github.com/giancarlosisasi/labdash/internal/tui/preview"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

func TestMain(m *testing.M) { harness.Main(m) }

// referenceWidth is the width every screen in research/16-screens-and-flows.md
// is drawn at, and the width these goldens are recorded at. A golden test
// without a stated width is a flake waiting for a different CI runner.
const referenceWidth = 120

// THM-13.T1. Prevents: a theme author having to guess. This is also what a
// rendering bug report is asked to attach, so every tier and both font modes
// are recorded.
func TestTHM13_T1_TheSheetRendersAtEveryTierAndFontMode(t *testing.T) {
	t.Parallel()

	for _, tier := range termcap.Tiers() {
		for _, icons := range []theme.IconSetting{theme.IconsUnicode, theme.IconsASCII} {
			t.Run(tier.Name+"/"+string(icons), func(t *testing.T) {
				t.Parallel()

				sheet := sheetFor(t, theme.Options{
					Env: tier.Terminal, Icons: icons,
				}, referenceWidth)

				out := sheet.Render()
				golden.Assert(t, termcap.GoldenName("preview", tier.Name, string(icons)), out)
				requireNoLineExceeds(t, out, referenceWidth)
			})
		}
	}
}

// THM-13.T1, the theme half. Prevents: a shipped theme that renders in the
// palette table and nowhere else.
func TestTHM13_T1_EveryShippedThemeRenders(t *testing.T) {
	t.Parallel()

	for _, name := range theme.Names() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sheet := sheetFor(t, theme.Options{
				Name: name, Env: termcap.TrueColor, Icons: theme.IconsUnicode,
			}, referenceWidth)

			out := sheet.Render()
			golden.Assert(t, "preview.theme."+name, out)
			requireNoLineExceeds(t, out, referenceWidth)

			// Every token, every status and every blocker is on the sheet.
			plain := ansi.Strip(out)
			for _, r := range theme.Roles() {
				require.Contains(t, plain, string(r.Token))
			}
			for _, s := range theme.PipelineStatuses() {
				require.Contains(t, plain, s.Glyph.Word)
			}
		})
	}
}

// THM-05.T1. Prevents: "usable at sixteen colours" being an aspiration rather
// than a fact. The same sheet at all four tiers is the same layout — only the
// colour changes — so a tier cannot quietly lose a column.
func TestTHM05_T1_TheLayoutIsIdenticalAcrossTiers(t *testing.T) {
	t.Parallel()

	var reference string
	for _, tier := range termcap.Tiers() {
		out := sheetFor(t, theme.Options{
			Env: tier.Terminal, Icons: theme.IconsUnicode,
		}, referenceWidth).Render()

		requireNoLineExceeds(t, out, referenceWidth)

		// The first row names the tier on purpose, so it is the one row that
		// differs. Everything below it is the layout, and it must not.
		lines := strings.Split(ansi.Strip(out), "\n")
		body := strings.Join(lines[1:], "\n")

		if reference == "" {
			reference = body
			continue
		}
		require.Equal(t, reference, body,
			"the %s tier renders a different layout from truecolor", tier.Name)
	}
}

// THM-07.T1. Prevents: a fallback of a different width shifting every column
// right. This is the check that caught [x], ro and >>.
func TestTHM07_T1_ColumnBoundariesAreIdenticalInBothFontModes(t *testing.T) {
	t.Parallel()

	unicodeLines := strings.Split(ansi.Strip(sheetFor(t, theme.Options{
		Env: termcap.TrueColor, Icons: theme.IconsUnicode,
	}, referenceWidth).Render()), "\n")

	asciiLines := strings.Split(ansi.Strip(sheetFor(t, theme.Options{
		Env: termcap.TrueColor, Icons: theme.IconsASCII,
	}, referenceWidth).Render()), "\n")

	require.Equal(t, len(unicodeLines), len(asciiLines),
		"the two font modes produced a different number of rows")

	for i := range unicodeLines {
		require.Equal(t, ansi.StringWidth(unicodeLines[i]), ansi.StringWidth(asciiLines[i]),
			"row %d is a different width:\n  unicode %q\n  ascii   %q",
			i+1, unicodeLines[i], asciiLines[i])
	}
}

// ACC-04.T1. Prevents: an "ASCII mode" that still emits box-drawing characters.
func TestACC04_T1_ASCIIModeEmitsNoByteAbove0x7F(t *testing.T) {
	t.Parallel()

	out := sheetFor(t, theme.Options{
		Env: termcap.TrueColor, Icons: theme.IconsASCII,
	}, referenceWidth).Render()

	for i, c := range []byte(ansi.Strip(out)) {
		require.Less(t, c, byte(0x80),
			"byte %d is above 0x7F, so ASCII mode is not ASCII", i)
	}
}

// ACC-01.T1. Prevents: the most common accessibility failure in TUIs, and the
// easiest to regress. With the colour gone, every state is still identifiable
// from its glyph and its word.
func TestACC01_T1_EveryStateSurvivesTheColourBeingStripped(t *testing.T) {
	t.Parallel()

	out := sheetFor(t, theme.Options{
		Env: termcap.Monochrome, Icons: theme.IconsUnicode,
	}, referenceWidth).Render()

	require.Empty(t, ansi.Strip(out) != out && strings.Contains(out, "\x1b[38"),
		"a colour sequence survived NO_COLOR")

	plain := ansi.Strip(out)

	// Each state's word is on the sheet, and no two states read the same, so
	// the row a user is looking at can be named without seeing a colour.
	seen := map[string]string{}
	for _, s := range append(theme.PipelineStatuses(), theme.Markers()...) {
		require.Contains(t, plain, s.Glyph.Word,
			"%s has no word on the sheet, so it dies with the colour", s.State)
		if first, ok := seen[s.Glyph.Word]; ok && first != s.State {
			// Two vocabularies may share a word; two states within one may not,
			// which the theme package asserts. Here we only record the overlap.
			continue
		}
		seen[s.Glyph.Word] = s.State
	}

	// And the blockers, which are the "why is this stuck" half of the same
	// promise.
	for _, b := range theme.Blockers() {
		phrase := strings.ReplaceAll(b.Phrase, "%d", "2")
		require.Contains(t, plain, phrase, "the %s blocker has no phrase", b.Value)
	}
}

// Prevents: a narrow terminal smearing. The sheet reflows down to 50 columns by
// dropping columns from the right; it never wraps a row or clips without an
// ellipsis.
func TestTheSheetReflowsDownToFiftyColumns(t *testing.T) {
	t.Parallel()

	for _, width := range []int{50, 56, 58, 74, 88, 100, 120, 160} {
		t.Run(widthName(width), func(t *testing.T) {
			t.Parallel()

			out := sheetFor(t, theme.Options{
				Env: termcap.TrueColor, Icons: theme.IconsUnicode,
			}, width).Render()

			requireNoLineExceeds(t, out, width)
			require.Contains(t, ansi.Strip(out), "passed",
				"the pipeline vocabulary vanished at %d columns", width)
		})
	}

	golden.Assert(t, "preview.narrow", sheetFor(t, theme.Options{
		Env: termcap.TrueColor, Icons: theme.IconsUnicode,
	}, preview.MinWidth).Render())
}

// Prevents: a terminal below the floor producing a smeared sheet instead of an
// answer. The notice names the current width, the required width, and a way out.
func TestBelowTheFloorTheSheetSaysWhatItNeeds(t *testing.T) {
	t.Parallel()

	out := sheetFor(t, theme.Options{Env: termcap.TrueColor}, preview.MinWidth-1).Render()

	require.Contains(t, out, "needs 50 columns")
	require.Contains(t, out, "is 49")
	require.Contains(t, out, "Widen it")
	require.NotContains(t, out, "passed", "the sheet was drawn anyway")
}

// THM-04.T2, on screen. Prevents: an override being silently accepted with no
// way for the user to see what it cost them.
func TestAnOverriddenColourIsMarkedAndItsRatioReported(t *testing.T) {
	t.Parallel()

	// The override is another token's own value rather than a hex literal:
	// THM-01.T1 forbids a colour literal outside the theme package, and that
	// rule binds a test as much as it binds a component. Painting status.error
	// with an overlay background is a choice nobody would make, which is
	// exactly what a report-do-not-refuse test needs.
	shipped := sheetFor(t, theme.Options{Env: termcap.TrueColor}, referenceWidth).Theme

	out := sheetFor(t, theme.Options{
		Env: termcap.TrueColor, Icons: theme.IconsUnicode,
		Colors: map[theme.Token]string{
			theme.StatusError: shipped.Authored(theme.BgOverlay),
		},
	}, referenceWidth).Render()

	plain := ansi.Strip(out)
	require.Contains(t, plain, "*status.error", "the overridden token is not marked")
	require.Contains(t, plain, "a colour you set")
	require.Contains(t, plain, "status.error on bg.base")
	require.Contains(t, plain, "needs 4.50:1")

	golden.Assert(t, "preview.override", out)
	requireNoLineExceeds(t, out, referenceWidth)
}

// Prevents: a theme whose contrast cannot be computed passing in silence. The
// sheet says why rather than printing nothing.
func TestAThemeWithNoComputableContrastSaysSo(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"ansi", "mono"} {
		sheet := sheetFor(t, theme.Options{Name: name, Env: termcap.TrueColor}, referenceWidth)
		out := ansi.Strip(sheet.Render())

		reason := sheet.Theme.Policy().Reason
		require.NotEmpty(t, reason, "%s asserts no contrast and gives no reason", name)

		require.Contains(t, out, "Contrast")
		require.NotContains(t, out, "pairs measured")

		// The reason is wrapped to the sheet's width, so it is checked by its
		// words rather than as one line.
		for _, word := range strings.Fields(reason) {
			require.Contains(t, out, word,
				"%s does not print the reason it asserts no ratio", name)
		}
	}
}

func sheetFor(t *testing.T, opts theme.Options, width int) preview.Sheet {
	t.Helper()

	th, err := theme.New(opts)
	require.NoError(t, err)
	return preview.Sheet{Theme: th, Width: width}
}

func requireNoLineExceeds(t *testing.T, out string, width int) {
	t.Helper()

	for i, line := range strings.Split(ansi.Strip(out), "\n") {
		require.LessOrEqual(t, ansi.StringWidth(line), width,
			"row %d is %d cells wide in a %d-column terminal: %q",
			i+1, ansi.StringWidth(line), width, line)
	}
}

func widthName(w int) string { return strconv.Itoa(w) + "col" }
