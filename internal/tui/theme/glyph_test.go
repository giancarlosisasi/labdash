package theme_test

import (
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/width"

	"github.com/giancarlosisasi/labdash/internal/testsupport/termcap"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// vocabularies is every status vocabulary the product renders, by the name a
// failure reports.
func vocabularies() map[string][]theme.Status {
	return map[string][]theme.Status{
		"pipeline": theme.PipelineStatuses(),
		"markers":  theme.Markers(),
	}
}

// THM-07.T2. Prevents: reintroducing a wide or patched-font glyph. The set is
// width-safe today and one careless addition undoes it — a glyph drawn two
// cells where one was measured shifts every column after it.
func TestTHM07_T2_EveryGlyphIsOneSafeCell(t *testing.T) {
	t.Parallel()

	for name, vocabulary := range vocabularies() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, s := range vocabulary {
				g := s.Glyph

				require.Equal(t, 1, utf8.RuneCountInString(g.Unicode),
					"%s: the glyph is not a single rune", s.State)
				require.Equal(t, 1, ansi.StringWidth(g.Unicode),
					"%s: the glyph does not measure one cell", s.State)

				r, _ := utf8.DecodeRuneInString(g.Unicode)
				switch width.LookupRune(r).Kind() {
				case width.EastAsianWide, width.EastAsianFullwidth:
					t.Errorf("%s: U+%04X is East Asian Wide or Fullwidth, so it draws "+
						"two cells and shifts every column after it", s.State, r)
				}
				require.False(t, unicode.Is(unicode.Co, r),
					"%s: U+%04X is a private-use codepoint, which needs a patched font. "+
						"There is no Nerd Font tier — research/17-design-system.md §3.1.1",
					s.State, r)

				require.Equal(t, 1, utf8.RuneCountInString(g.ASCII),
					"%s: the ASCII fallback is not exactly one rune. "+
						"This caught [x], ro and >>", s.State)
				require.Equal(t, 1, ansi.StringWidth(g.ASCII),
					"%s: the ASCII fallback does not measure one cell", s.State)
				for _, b := range []byte(g.ASCII) {
					require.Less(t, b, byte(0x80),
						"%s: the ASCII fallback has a byte above 0x7F", s.State)
				}

				require.NotEmpty(t, g.Word, "%s: no word, so the state dies with the colour", s.State)
			}
		})
	}
}

// Task 4.1a, and the "every state is a triple" requirement. Prevents: two
// states that are indistinguishable once colour is stripped. A glyph may
// repeat; a word may not.
func TestNoWordRepeatsWithinAVocabulary(t *testing.T) {
	t.Parallel()

	for name, vocabulary := range vocabularies() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			seen := map[string]string{}
			for _, s := range vocabulary {
				if first, ok := seen[s.Glyph.Word]; ok {
					t.Errorf("%s and %s both read %q. With the colour stripped they are "+
						"the same state", first, s.State, s.Glyph.Word)
				}
				seen[s.Glyph.Word] = s.State
			}
		})
	}
}

// Prevents: a glyph repeat being read as an accident. Three glyphs are shared
// on purpose and the word is what separates the states; this records which,
// so that a fourth is a decision rather than a slip.
func TestGlyphsRepeatWhereTheWordSeparates(t *testing.T) {
	t.Parallel()

	byGlyph := map[string][]string{}
	for _, s := range theme.PipelineStatuses() {
		byGlyph[s.Glyph.Unicode] = append(byGlyph[s.Glyph.Unicode], s.State)
	}

	require.Equal(t, []string{"pending", "preparing"}, byGlyph["◌"])
	require.Equal(t, []string{"canceled", "no-pipeline"}, byGlyph["⊘"])
	require.Equal(t, []string{"failed", "allow-failure-failed"}, byGlyph["✖"])
}

// Prevents: rendering an allow-failure job in red. It is the most common
// CI-dashboard mistake and it trains people to ignore red, which is the one
// colour that must never be ignored.
func TestAnAllowFailureJobIsAWarningNotAnError(t *testing.T) {
	t.Parallel()

	allowed := statusNamed(t, "allow-failure-failed")
	require.Equal(t, theme.StatusWarning, allowed.Token)
	require.Equal(t, "failed (allowed)", allowed.Glyph.Word)

	real := statusNamed(t, "failed")
	require.Equal(t, theme.StatusError, real.Token)
	require.Equal(t, allowed.Glyph.Unicode, real.Glyph.Unicode,
		"the two failures should share a glyph; the colour and the word separate them")
}

// Prevents: a GitLab pipeline state arriving with no rendering. Fifteen enum
// values map onto fourteen states — the two waiting values render as one,
// because a user can do nothing different about either.
func TestEveryGitLabPipelineStateIsMapped(t *testing.T) {
	t.Parallel()

	want := []string{
		"SUCCESS", "FAILED", "RUNNING", "PENDING", "CREATED", "PREPARING",
		"WAITING_FOR_RESOURCE", "WAITING_FOR_CALLBACK", "MANUAL", "SCHEDULED",
		"CANCELING", "CANCELED", "SKIPPED",
	}

	mapped := map[string]string{}
	for _, s := range theme.PipelineStatuses() {
		for _, from := range s.From {
			require.NotContains(t, mapped, from, "%s is mapped twice", from)
			mapped[from] = s.State
		}
	}

	for _, enum := range want {
		require.Contains(t, mapped, enum, "%s renders as nothing", enum)
	}
	require.Len(t, mapped, len(want))
	require.Len(t, theme.PipelineStatuses(), 14,
		"thirteen GitLab states plus two of ours, with the two waiting values sharing one")
	require.Equal(t, "waiting", mapped["WAITING_FOR_RESOURCE"])
	require.Equal(t, "waiting", mapped["WAITING_FOR_CALLBACK"])
}

// Prevents: a detailedMergeStatus value rendering as the raw enum. "Why is this
// stuck" is the question the dashboard exists to answer, and BROKEN_STATUS is
// not an answer.
func TestEveryBlockerHasAPhrase(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, b := range theme.Blockers() {
		require.NotEmpty(t, b.Phrase, "%s has no phrase", b.Value)
		require.NotEqual(t, b.Value, b.Phrase, "%s renders as its own enum name", b.Value)
		require.False(t, seen[b.Value], "%s is mapped twice", b.Value)
		seen[b.Value] = true
	}

	require.Contains(t, seen, "MERGEABLE")
	require.Contains(t, seen, "CONFLICT")
	require.Contains(t, seen, "DISCUSSIONS_NOT_RESOLVED")
}

// THM-07.T3. Prevents: three ambiguous-width glyphs rendering two cells wide in
// a CJK terminal while we measure one, which shifts every column after them.
// This is what ACC-09 promises will not happen.
func TestTHM07_T3_AutoResolvesByLocale(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		env     theme.Environment
		setting theme.IconSetting
		want    theme.IconTier
	}{
		{"CJK locale", termcap.CJK, theme.IconsAuto, theme.ASCII},
		{"Chinese", termcap.TrueColor.With("LANG", "zh_CN.UTF-8"), theme.IconsAuto, theme.ASCII},
		{"Korean", termcap.TrueColor.With("LANG", "ko_KR.UTF-8"), theme.IconsAuto, theme.ASCII},
		{"Japanese by codeset", termcap.TrueColor.With("LANG", "ja.EUC-JP"), theme.IconsAuto, theme.ASCII},
		{"en_US.UTF-8", termcap.TrueColor, theme.IconsAuto, theme.Unicode},
		{"Spanish", termcap.TrueColor.With("LANG", "es_PE.UTF-8"), theme.IconsAuto, theme.Unicode},

		// LC_ALL wins over LANG, as POSIX says.
		{"LC_ALL overrides LANG", termcap.CJK.With("LC_ALL", "en_US.UTF-8"), theme.IconsAuto, theme.Unicode},
		{"LC_CTYPE overrides LANG", termcap.TrueColor.With("LC_CTYPE", "ja_JP.UTF-8"), theme.IconsAuto, theme.ASCII},

		// The setting overrides the locale in both directions.
		{"unicode forced on CJK", termcap.CJK, theme.IconsUnicode, theme.Unicode},
		{"ascii forced on en_US", termcap.TrueColor, theme.IconsASCII, theme.ASCII},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, theme.ResolveIcons(tc.setting, tc.env))

			th := build(t, theme.Options{Env: tc.env, Icons: tc.setting})
			require.Equal(t, tc.want, th.Icons)
		})
	}
}

// ACC-04. Prevents: an "ASCII mode" that still emits box-drawing characters.
func TestASCIIModeEmitsNoByteAbove0x7F(t *testing.T) {
	t.Parallel()

	th := build(t, theme.Options{Env: termcap.TrueColor, Icons: theme.IconsASCII})

	var all []string
	for _, v := range vocabularies() {
		for _, s := range v {
			all = append(all, th.Glyph(s.Glyph))
		}
	}
	b := th.Borders()
	all = append(all, b.TopLeft, b.TopRight, b.BottomLeft, b.BottomRight,
		b.Horizontal, b.Vertical, th.Ellipsis(), th.ReducedMotionMarker())
	all = append(all, th.SpinnerFrames()...)

	for _, s := range all {
		for _, c := range []byte(s) {
			require.Less(t, c, byte(0x80),
				"ASCII mode emitted a byte above 0x7F in %q", s)
		}
	}
}

// Prevents: a spinner that is a printed character rather than an animation. A
// static marker cannot say "this is live", so the spinner has real frames and
// the reduced-motion path substitutes a marker while the words keep carrying
// the state.
func TestTheSpinnerHasRealFrames(t *testing.T) {
	t.Parallel()

	unicodeTheme := build(t, theme.Options{Env: termcap.TrueColor, Icons: theme.IconsUnicode})
	require.Len(t, unicodeTheme.SpinnerFrames(), 10,
		"ten braille frames at ten frames a second is one revolution a second")

	asciiTheme := build(t, theme.Options{Env: termcap.TrueColor, Icons: theme.IconsASCII})
	require.Equal(t, []string{"|", "/", "-", "\\"}, asciiTheme.SpinnerFrames())

	for _, th := range []theme.Theme{unicodeTheme, asciiTheme} {
		seen := map[string]bool{}
		for _, f := range th.SpinnerFrames() {
			require.Equal(t, 1, ansi.StringWidth(f), "a spinner frame is not one cell")
			require.False(t, seen[f], "a spinner frame repeats, so the animation stutters")
			seen[f] = true
		}
		require.NotEmpty(t, th.ReducedMotionMarker())
	}
}

// Prevents: a focused border changing geometry. Colour changes, geometry never
// does — a border that thickens on focus shifts every column beside it.
func TestBordersKeepTheirGeometryInBothModes(t *testing.T) {
	t.Parallel()

	for _, icons := range []theme.IconSetting{theme.IconsUnicode, theme.IconsASCII} {
		b := build(t, theme.Options{Env: termcap.TrueColor, Icons: icons}).Borders()
		for _, part := range []string{
			b.TopLeft, b.TopRight, b.BottomLeft, b.BottomRight, b.Horizontal, b.Vertical,
		} {
			require.Equal(t, 1, ansi.StringWidth(part),
				"a border character is not one cell in %s mode", icons)
		}
	}
}

func statusNamed(t *testing.T, state string) theme.Status {
	t.Helper()
	for _, s := range theme.PipelineStatuses() {
		if s.State == state {
			return s
		}
	}
	t.Fatalf("no pipeline status named %q", state)
	return theme.Status{}
}
