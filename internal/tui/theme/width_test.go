package theme_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// hostile is cell content that has drifted a table somewhere. Each entry is a
// class of failure rather than a random string.
var hostile = []string{
	"platform/ingest",
	"数据平台/采集服务",                 // CJK: two cells per rune
	"プラットフォーム",                    // kana
	"플랫폼/수집",                       // hangul
	"deploy 🚀 to staging",           // emoji, two cells
	"👩‍💻 pair review",                // ZWJ sequence: one grapheme, several runes
	"équipe",                   // combining acute
	"á́́bc",          // stacked combining marks
	"🇵🇪 lima",                        // regional indicator pair
	"tab\there",                      // a tab, which no cell should carry
	"\x1b[31malready styled\x1b[0m",  // an escape sequence already in the string
	"",                               // empty
	"—",                              // em dash
	"«guillemets»",
	strings.Repeat("é", 40),
	strings.Repeat("宽", 40),
}

// ACC-09.T1. Prevents: the bug that makes tables drift for anyone with a
// non-Latin project name. A cell renders at exactly the width it was allotted,
// whatever is in it.
func TestACC09_T1_RenderedWidthAlwaysEqualsTheAllottedWidth(t *testing.T) {
	t.Parallel()

	for _, tail := range []string{"…", "..."} {
		for _, content := range hostile {
			for _, w := range []int{1, 2, 3, 5, 8, 12, 20, 40, 80} {
				got := theme.Pad(content, w, tail)
				require.Equal(t, w, theme.Width(got),
					"Pad(%q, %d, %q) rendered %d cells", content, w, tail, theme.Width(got))

				got = theme.PadLeft(content, w, tail)
				require.Equal(t, w, theme.Width(got),
					"PadLeft(%q, %d, %q) rendered %d cells", content, w, tail, theme.Width(got))

				got = theme.Center(content, w, tail)
				require.Equal(t, w, theme.Width(got),
					"Center(%q, %d, %q) rendered %d cells", content, w, tail, theme.Width(got))
			}
		}
	}
}

// ACC-09.T1, the truncation half. Prevents: a cut that overflows its column by
// one because a wide rune straddled the boundary.
func TestACC09_T1_TruncationNeverExceedsItsWidth(t *testing.T) {
	t.Parallel()

	for _, content := range hostile {
		for w := 0; w <= 24; w++ {
			got := theme.Truncate(content, w, "…")
			require.LessOrEqual(t, theme.Width(got), w,
				"Truncate(%q, %d) rendered %d cells", content, w, theme.Width(got))
			require.True(t, utf8.ValidString(got),
				"Truncate(%q, %d) cut inside a rune", content, w)
		}
	}
}

// Prevents: a hard clip. Truncation always says that something was removed,
// which is the difference between a title you can trust and a title you cannot.
func TestTruncationEndsInTheEllipsis(t *testing.T) {
	t.Parallel()

	const title = "Add a retry budget to the ingest worker"

	require.Equal(t, title, theme.Truncate(title, 60, "…"),
		"a string that fits was changed")

	short := theme.Truncate(title, 20, "…")
	require.True(t, strings.HasSuffix(short, "…"), "got %q", short)
	require.Equal(t, 20, theme.Width(short))
}

// Prevents: measuring an already-styled string by its bytes, which counts every
// escape sequence as content and drops a column.
func TestWidthIgnoresEscapeSequences(t *testing.T) {
	t.Parallel()

	th := build(t, theme.Options{Tier: theme.TrueColor})
	styled := th.Style(th.StatusError).Render("failed")

	require.Equal(t, 6, theme.Width(styled))
	require.Greater(t, len(styled), 6, "the test string carries no escape sequence")
}

// THM-07.T1, the column half. Prevents: a fallback of a different width
// shifting every column right. The same row rendered in both icon tiers ends at
// the same cell.
func TestTHM07_T1_ColumnBoundariesAreIdenticalInBothIconTiers(t *testing.T) {
	t.Parallel()

	unicodeTheme := build(t, theme.Options{Icons: theme.IconsUnicode})
	asciiTheme := build(t, theme.Options{Icons: theme.IconsASCII})

	for _, s := range theme.PipelineStatuses() {
		u := theme.Center(unicodeTheme.Glyph(s.Glyph), 3, "…")
		a := theme.Center(asciiTheme.Glyph(s.Glyph), 3, "...")
		require.Equal(t, theme.Width(u), theme.Width(a),
			"%s occupies a different number of cells in the two tiers", s.State)
	}

	// And the whole row, not only the glyph column.
	row := func(th theme.Theme, s theme.Status) string {
		return theme.Center(th.Glyph(s.Glyph), 3, th.Ellipsis()) +
			strings.Repeat(" ", theme.SpaceNormal) +
			theme.Pad(s.Glyph.Word, 18, th.Ellipsis()) +
			strings.Repeat(" ", theme.SpaceNormal) +
			theme.Pad(string(s.Token), 16, th.Ellipsis())
	}
	for _, s := range theme.PipelineStatuses() {
		require.Equal(t,
			ansi.StringWidth(row(unicodeTheme, s)),
			ansi.StringWidth(row(asciiTheme, s)),
			"the %s row is a different width in the two tiers", s.State)
	}
}
