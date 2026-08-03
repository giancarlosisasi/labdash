package theme

import (
	"strings"
)

// An IconSetting is what the user asked for. It is authored in settings.yml as
// theme.icons and accepted by `theme preview --icons`.
type IconSetting string

const (
	// IconsAuto reads the locale and chooses. It is the default.
	IconsAuto IconSetting = "auto"
	// IconsUnicode forces the glyphs on, for somebody who knows their terminal
	// treats East-Asian-Ambiguous characters as one cell.
	IconsUnicode IconSetting = "unicode"
	// IconsASCII forces the fallbacks, for serial consoles, CI logs and
	// restrictive corporate terminals.
	IconsASCII IconSetting = "ascii"
)

// IconSettings is every accepted value, for a flag's help text and its
// validation.
func IconSettings() []IconSetting { return []IconSetting{IconsAuto, IconsUnicode, IconsASCII} }

// An IconTier is what the setting resolved to. There are exactly two, and no
// patched or Nerd font is ever required — decided 2026-08-02, see
// research/17-design-system.md §3.1.1.
type IconTier uint8

const (
	// Unicode uses the glyph column of the vocabulary.
	Unicode IconTier = iota
	// ASCII uses the fallback column. No byte above 0x7F reaches the screen.
	ASCII
)

// String is the name used in a golden file and in `theme preview`.
func (i IconTier) String() string {
	if i == ASCII {
		return "ascii"
	}
	return "unicode"
}

// A Glyph is one state's whole appearance: the Unicode form, the ASCII
// fallback, and the word that carries the state when colour is gone.
//
// The three travel together in one value so that rendering a status without its
// word is structurally difficult rather than a review comment. That is ACC-01,
// and it is the reason this is a struct and not three parallel maps.
//
// Invariants, asserted by THM-07.T2:
//   - Unicode is exactly one rune, one cell, not East-Asian Wide or Fullwidth,
//     and not a private-use codepoint.
//   - ASCII is exactly one rune, one cell, and below 0x80.
//
// A fallback of a different width from its glyph shifts every column after it,
// which is a layout bug rather than a cosmetic one.
type Glyph struct {
	Unicode string
	ASCII   string
	Word    string
}

// In returns the form for one icon tier.
func (g Glyph) In(tier IconTier) string {
	if tier == ASCII {
		return g.ASCII
	}
	return g.Unicode
}

// A Status is one rendered state: its glyph triple, the colour role it takes,
// and the GitLab enum values that arrive as it.
type Status struct {
	// State is labdash's name for it, and the key a test refers to.
	State string
	// Glyph carries the glyph, the fallback and the word.
	Glyph Glyph
	// Token is the colour role. Meaning never rides on the colour alone; the
	// token is the third leg of the triple, not the first.
	Token Token
	// From lists GitLab's own enum values that render as this state. It is
	// empty for the states that are labdash's rather than GitLab's.
	//
	// The mapping is many-to-one on purpose: GitLab distinguishes waiting for a
	// resource from waiting for a callback, and a user can do nothing different
	// about either, so both render as one state with one word.
	From []string
}

// Word is the colour-independent form.
func (s Status) Word() string { return s.Glyph.Word }

// PipelineStatuses is the whole pipeline and job vocabulary: fifteen GitLab
// enum values rendering as fourteen states.
//
// research/17-design-system.md §3.1. An allow-failure job that failed carries
// the warning role, never the error role — rendering it red is the most common
// CI-dashboard mistake and it trains people to ignore red.
func PipelineStatuses() []Status {
	return []Status{
		{State: "passed", Token: StatusSuccess, From: []string{"SUCCESS"},
			Glyph: Glyph{"✔", "+", "passed"}},
		{State: "failed", Token: StatusError, From: []string{"FAILED"},
			Glyph: Glyph{"✖", "x", "failed"}},
		{State: "running", Token: StatusRunning, From: []string{"RUNNING"},
			Glyph: Glyph{"●", ">", "running"}},
		{State: "pending", Token: StatusPending, From: []string{"PENDING"},
			Glyph: Glyph{"◌", ".", "pending"}},
		{State: "created", Token: StatusNeutral, From: []string{"CREATED"},
			Glyph: Glyph{"○", "-", "created"}},
		{State: "preparing", Token: StatusPending, From: []string{"PREPARING"},
			Glyph: Glyph{"◌", ".", "preparing"}},
		{State: "waiting", Token: StatusPending,
			From:  []string{"WAITING_FOR_RESOURCE", "WAITING_FOR_CALLBACK"},
			Glyph: Glyph{"◍", "~", "waiting"}},
		{State: "manual", Token: StatusWarning, From: []string{"MANUAL"},
			Glyph: Glyph{"⏸", "!", "manual"}},
		{State: "scheduled", Token: StatusPending, From: []string{"SCHEDULED"},
			Glyph: Glyph{"◔", "@", "scheduled"}},
		{State: "canceling", Token: StatusNeutral, From: []string{"CANCELING"},
			Glyph: Glyph{"◐", "/", "canceling"}},
		{State: "canceled", Token: StatusNeutral, From: []string{"CANCELED"},
			Glyph: Glyph{"⊘", "/", "canceled"}},
		{State: "skipped", Token: StatusNeutral, From: []string{"SKIPPED"},
			Glyph: Glyph{"»", "s", "skipped"}},

		// The two states that are ours, not GitLab's.
		{State: "no-pipeline", Token: TextMuted,
			Glyph: Glyph{"⊘", "-", "no pipeline"}},
		{State: "allow-failure-failed", Token: StatusWarning,
			Glyph: Glyph{"✖", "w", "failed (allowed)"}},
	}
}

// Markers is the rest of the vocabulary: the states that are not a pipeline.
// research/17-design-system.md §3.3.
func Markers() []Status {
	return []Status{
		{State: "approved", Token: StatusSuccess, Glyph: Glyph{"✓", "v", "approved"}},
		{State: "approval-needed", Token: StatusWarning, Glyph: Glyph{"◦", "o", "needs approval"}},
		{State: "merge-train", Token: AccentSecondary, Glyph: Glyph{"≡", "=", "on the train"}},
		{State: "draft", Token: TextMuted, Glyph: Glyph{"▲", "^", "draft"}},
		{State: "selected", Token: AccentPrimary, Glyph: Glyph{"→", ">", "selected"}},
		{State: "multi-selected", Token: AccentPrimary, Glyph: Glyph{"▣", "#", "picked"}},
		{State: "changed", Token: AccentPrimary, Glyph: Glyph{"✱", "*", "changed"}},
		{State: "offline", Token: StatusWarning, Glyph: Glyph{"⚠", "!", "offline"}},
		{State: "read-only", Token: TextMuted, Glyph: Glyph{"⊙", "r", "read-only"}},
		{State: "stale", Token: StatusNeutral, Glyph: Glyph{"≈", "~", "stale"}},
	}
}

// A Blocker is one value of GitLab's detailedMergeStatus, rendered as the
// phrase that says why a merge request cannot merge.
//
// The phrase is the product: "why is this stuck" is the question the dashboard
// exists to answer, and the raw enum is not an answer. MRQ-09.T1 fails if any
// schema value is unmapped, and an unrecognised future value is title-cased and
// logged rather than dropped.
//
// research/17-design-system.md §3.2. The full 24-value table is filled from the
// pinned SDL in C04b, where the query that returns the field is generated; the
// entries here are the ones the design system publishes.
type Blocker struct {
	// Value is GitLab's enum name.
	Value string
	// Phrase is what the blockers column shows. Where it counts something, it
	// carries a %d that the row's own number fills.
	Phrase string
	// Token is the colour role.
	Token Token
}

// Blockers is the published subset of the detailedMergeStatus mapping.
func Blockers() []Blocker {
	return []Blocker{
		{"MERGEABLE", "ready", StatusSuccess},
		{"CONFLICT", "conflicts", StatusError},
		{"CI_MUST_PASS", "ci failed", StatusError},
		{"CI_STILL_RUNNING", "ci running", StatusRunning},
		{"NOT_APPROVED", "needs %d", StatusWarning},
		{"DISCUSSIONS_NOT_RESOLVED", "%d threads open", StatusWarning},
		{"DRAFT_STATUS", "draft", TextMuted},
		{"NEED_REBASE", "needs rebase", StatusWarning},
		{"BLOCKED_STATUS", "blocked by %d", StatusError},
		{"REQUESTED_CHANGES", "changes requested", StatusError},
		{"CHECKING", "checking…", TextMuted},
		{"UNCHECKED", "checking…", TextMuted},
		{"BROKEN_STATUS", "cannot merge", StatusError},
		{"NOT_OPEN", "closed", TextMuted},
		{"EXTERNAL_STATUS_CHECKS", "external check", StatusWarning},
		{"POLICIES_DENIED", "policy blocked", StatusError},
	}
}

// Borders is the frame vocabulary. Focus changes a border's colour and never
// its geometry: a border that thickens on focus shifts every column beside it.
//
// The four tees are where a rule meets the frame or a pane divider. In ASCII
// every one of them is "+", which is the same width and reads as a junction.
type Borders struct {
	TopLeft, TopRight, BottomLeft, BottomRight string
	Horizontal, Vertical                       string
	TeeLeft, TeeRight, TeeDown, TeeUp          string
}

// BordersFor returns the frame characters for one icon tier.
func BordersFor(tier IconTier) Borders {
	if tier == ASCII {
		return Borders{
			"+", "+", "+", "+", "-", "|",
			"+", "+", "+", "+",
		}
	}
	return Borders{
		"╭", "╮", "╰", "╯", "─", "│",
		"├", "┤", "┬", "┴",
	}
}

// SpinnerFrames is the animation shown while work is in flight, at ten frames
// a second. It is a real tick driven by the update loop, never a printed
// character that only looks busy.
//
// Under reduced motion the spinner is replaced by ReducedMotionMarker and the
// task line still names what is running, so the state survives the animation
// being switched off.
func SpinnerFrames(tier IconTier) []string {
	if tier == ASCII {
		return []string{"|", "/", "-", "\\"}
	}
	return []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
}

// ReducedMotionMarker is what stands in for a spinner when motion is off.
func ReducedMotionMarker(tier IconTier) string {
	if tier == ASCII {
		return "*"
	}
	return "•"
}

// Ellipsis is the truncation mark. Truncation is always this, never a hard
// clip.
//
// The one-cell rule in research/17-design-system.md §3.3 binds the status
// vocabulary, where a fallback of a different width shifts a column. The
// ellipsis is not in that vocabulary: it is the tail of a truncation whose
// total width is fixed either way, so ASCII mode spends two extra cells to say
// "there is more" unambiguously rather than ending a title in what reads as a
// full stop.
func Ellipsis(tier IconTier) string {
	if tier == ASCII {
		return "..."
	}
	return "…"
}

// ResolveIcons turns the setting into a tier, reading the locale when the
// setting is auto.
//
// auto reads the locale and only the locale. There is no font probe, because
// with no Nerd Font tier there is nothing to probe for, and every probe the
// competition ships is unreliable — some multiplexer and font combinations
// report a glyph they cannot draw.
//
// The real width risk is East Asian Ambiguous width. Nine glyphs in the
// vocabulary are EAW=A: one cell in a Western locale, two in a terminal
// configured ambiguous-width=double, which is normal in a CJK setup. We measure
// one and the terminal draws two, and every column after them shifts. So a CJK
// locale resolves to ascii. research/17-design-system.md §3.1.2.
func ResolveIcons(setting IconSetting, env Environment) IconTier {
	switch setting {
	case IconsUnicode:
		return Unicode
	case IconsASCII:
		return ASCII
	}

	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v, ok := env.Lookup(key)
		if !ok || v == "" {
			continue
		}
		if isCJKLocale(v) {
			return ASCII
		}
		return Unicode
	}
	return Unicode
}

// isCJKLocale reports whether a locale string names a language or an encoding
// that renders East-Asian-Ambiguous characters two cells wide.
func isCJKLocale(locale string) bool {
	l := strings.ToLower(locale)

	// Language subtag, before any territory or codeset.
	lang := l
	for _, sep := range []string{"_", "-", "."} {
		if i := strings.Index(lang, sep); i >= 0 {
			lang = lang[:i]
		}
	}
	switch lang {
	case "zh", "ja", "ko", "yue":
		return true
	}

	// Codeset, for a locale written without a language we recognise.
	for _, cs := range []string{"gb2312", "gbk", "gb18030", "big5", "euc-jp", "eucjp",
		"euc-kr", "euckr", "sjis", "shift_jis", "cp932", "cp936", "cp949", "cp950"} {
		if strings.Contains(l, cs) {
			return true
		}
	}
	return false
}
