// Package preview renders S-27: labdash's whole visual vocabulary, in the
// terminal the user is actually sitting at.
//
// It exists so that a theme, a colour tier or a glyph set can be judged before
// any feature depends on it, and so that every screen built after this one is
// judged the same way. It is also what a rendering bug report is asked to
// attach.
//
// It names semantic roles and never a colour. THM-01.T1 walks this package
// along with every other.
package preview

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// MinWidth is the narrowest terminal the sheet draws in. Below it the sheet
// says what it needs rather than smearing.
const MinWidth = 50

// A Sheet is one rendering of the vocabulary.
type Sheet struct {
	// Theme is what is being shown, already resolved to a colour tier and an
	// icon tier.
	Theme theme.Theme
	// Width is the terminal's width in cells.
	Width int
}

// Render returns the whole sheet, ending in a newline.
func (s Sheet) Render() string {
	if s.Width < MinWidth {
		return s.tooNarrow()
	}

	var b strings.Builder
	s.header(&b)
	s.tokens(&b)
	s.pipeline(&b)
	s.blockers(&b)
	s.markers(&b)
	s.contrast(&b)
	return b.String()
}

// tooNarrow names the current width and the required one, and offers the way
// out. Never "see the docs".
func (s Sheet) tooNarrow() string {
	return fmt.Sprintf(
		"The theme preview needs %d columns. This terminal is %d.\n"+
			"Widen it, or run `labdash theme preview | less -R`.\n",
		MinWidth, s.Width)
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

func (s Sheet) header(b *strings.Builder) {
	th := s.Theme

	title := s.bold().Render("labdash theme preview")
	meta := th.Style(th.TextMuted).Render(strings.Join([]string{
		th.Name, th.Tier.String(), th.Icons.String(),
	}, s.separator()))

	gap := s.Width - theme.Width(title) - theme.Width(meta)
	if gap < theme.SpaceNormal {
		// Too narrow for one line: the metadata takes its own row rather than
		// being cut.
		fmt.Fprintf(b, "%s\n%s\n", title, meta)
	} else {
		fmt.Fprintf(b, "%s%s%s\n", title, strings.Repeat(" ", gap), meta)
	}
}

// tokens draws the surfaces, the text tiers and the meaning colours through one
// column layout, so the three sections line up with each other.
func (s Sheet) tokens(b *strings.Builder) {
	cols := s.tokenColumns()

	groups := []struct {
		heading string
		kinds   []theme.Kind
	}{
		{"Surfaces and borders", []theme.Kind{theme.KindSurface, theme.KindDecoration, theme.KindUI}},
		{"Text", []theme.Kind{theme.KindText, theme.KindInverted}},
		{"Meaning", []theme.Kind{theme.KindMeaning}},
	}

	for _, g := range groups {
		s.heading(b, g.heading)
		s.headerRow(b, cols)
		for _, r := range theme.Roles() {
			if !containsKind(g.kinds, r.Kind) {
				continue
			}
			s.tokenRow(b, cols, r)
		}
	}
}

func (s Sheet) tokenRow(b *strings.Builder, cols []column, r theme.Role) {
	th := s.Theme
	value := r.Of(th)

	name := string(r.Token)
	if th.Overridden(r.Token) {
		name = "*" + name
	}

	cells := []cell{
		{text: name, style: th.Style(th.TextSecondary)},
		{text: th.Authored(r.Token), style: th.Style(th.TextMuted)},
		{text: s.swatch(), style: th.Style(value)},
		{text: s.ratio(r), style: th.Style(th.TextMuted)},
		{text: s.sample(r), style: th.Style(value)},
		{text: r.Use, style: th.Style(th.TextMuted)},
	}
	s.row(b, cols, cells)
}

func (s Sheet) pipeline(b *strings.Builder) {
	s.heading(b, "Pipeline and job status")
	cols := s.statusColumns(true)
	s.headerRow(b, cols)

	for _, st := range theme.PipelineStatuses() {
		s.statusRow(b, cols, st, strings.Join(st.From, " "))
	}
}

// markers draws the states that are not a pipeline. They carry no GitLab enum,
// so the FROM column is absent rather than present and empty.
func (s Sheet) markers(b *strings.Builder) {
	s.heading(b, "Markers")
	cols := s.statusColumns(false)
	s.headerRow(b, cols)

	for _, st := range theme.Markers() {
		s.statusRow(b, cols, st, "")
	}
}

func (s Sheet) statusRow(b *strings.Builder, cols []column, st theme.Status, from string) {
	th := s.Theme
	value := th.Color(st.Token)

	s.row(b, cols, []cell{
		{text: th.Glyph(st.Glyph), style: th.Style(value)},
		{text: st.Glyph.ASCII, style: th.Style(th.TextMuted)},
		{text: st.Glyph.Word, style: th.Style(value)},
		{text: string(st.Token), style: th.Style(th.TextMuted)},
		{text: from, style: th.Style(th.TextMuted)},
	})
}

func (s Sheet) blockers(b *strings.Builder) {
	s.heading(b, "Merge blockers")
	cols := s.blockerColumns()
	s.headerRow(b, cols)

	th := s.Theme
	for _, bl := range theme.Blockers() {
		value := th.Color(bl.Token)
		s.row(b, cols, []cell{
			{text: fillCount(bl.Phrase), style: th.Style(value)},
			{text: string(bl.Token), style: th.Style(th.TextMuted)},
			{text: bl.Value, style: th.Style(th.TextMuted)},
		})
	}
}

// contrast says whether the theme's own promise holds, in this terminal, with
// the numbers rather than an assurance.
func (s Sheet) contrast(b *strings.Builder) {
	th := s.Theme
	policy := th.Policy()

	s.heading(b, "Contrast")

	if policy.TextRequired == 0 {
		s.wrapped(b, policy.Reason, th.Style(th.TextMuted))
		b.WriteString("\n")
		return
	}

	pairs := th.Contrast()
	var failed []theme.Pair
	var overridden []theme.Pair
	for _, p := range pairs {
		if !p.Meets() {
			failed = append(failed, p)
		}
		if p.Overridden {
			overridden = append(overridden, p)
		}
	}

	if len(pairs) > 0 {
		s.wrapped(b, fmt.Sprintf(
			"%d pairs measured. Text needs %s, interface elements %s.",
			len(pairs), theme.FormatRatio(policy.TextRequired),
			theme.FormatRatio(policy.UIRequired)), th.Style(th.TextMuted))

		s.wrapped(b, "Lowest: "+pairs[0].String(),
			th.Style(colourFor(th, len(failed) == 0)))
	}

	if len(failed) == 0 {
		s.wrapped(b, "Every pair clears.", th.Style(th.StatusSuccess))
	}
	for _, p := range failed {
		s.wrapped(b, "Below the threshold: "+p.String(), th.Style(th.StatusWarning))
	}

	// A colour the user replaced is reported on, never refused. It is their
	// machine; the tool's job is to say what the choice produced.
	if len(overridden) > 0 {
		b.WriteString("\n")
		s.wrapped(b, "* marks a colour you set. Its ratio is reported, not enforced:",
			th.Style(th.TextMuted))
		for _, p := range overridden {
			mark := th.Style(th.TextMuted)
			if !p.Meets() {
				mark = th.Style(th.StatusWarning)
			}
			s.wrapped(b, p.String(), mark)
		}
	}

	b.WriteString("\n")
}

// ---------------------------------------------------------------------------
// The column system
// ---------------------------------------------------------------------------

type alignment uint8

const (
	alignLeft alignment = iota
	alignCentre
)

// A column has a fixed width and the sheet width below which it drops. Column
// order is ours and permanent; a narrow terminal loses columns from the right,
// never rearranges them.
type column struct {
	header  string
	width   int
	align   alignment
	minimum int // the sheet width at or above which this column is drawn
	grows   bool
}

type cell struct {
	text  string
	style lipgloss.Style
}

const indent = "  "

func (s Sheet) tokenColumns() []column {
	return s.fit([]column{
		{header: "TOKEN", width: 17},
		{header: "VALUE", width: 9},
		{header: "SWATCH", width: 6, align: alignCentre},
		{header: "RATIO", width: 9, minimum: 52},
		{header: "SAMPLE", width: 18, minimum: 74},
		{header: "USED FOR", width: 26, minimum: 100, grows: true},
	})
}

func (s Sheet) statusColumns(withFrom bool) []column {
	cols := []column{
		{header: "GLYPH", width: 5, align: alignCentre},
		{header: "ASCII", width: 5, align: alignCentre, minimum: 58},
		{header: "WORD", width: 18},
		{header: "TOKEN", width: 16},
	}
	if withFrom {
		cols = append(cols, column{header: "FROM", width: 24, minimum: 88, grows: true})
	}
	return s.fit(cols)
}

func (s Sheet) blockerColumns() []column {
	return s.fit([]column{
		{header: "PHRASE", width: 20},
		{header: "TOKEN", width: 16},
		{header: "detailedMergeStatus", width: 26, minimum: 74, grows: true},
	})
}

// fit drops the columns this width cannot carry and lets the last survivor grow
// into whatever is left.
func (s Sheet) fit(cols []column) []column {
	var kept []column
	for _, c := range cols {
		if c.minimum > 0 && s.Width < c.minimum {
			continue
		}
		kept = append(kept, c)
	}

	used := len(indent)
	for i, c := range kept {
		used += c.width
		if i > 0 {
			used += theme.SpaceNormal
		}
	}
	if slack := s.Width - used; slack > 0 && len(kept) > 0 && kept[len(kept)-1].grows {
		kept[len(kept)-1].width += slack
	}

	return kept
}

func (s Sheet) headerRow(b *strings.Builder, cols []column) {
	cells := make([]cell, len(cols))
	for i, c := range cols {
		cells[i] = cell{text: c.header, style: s.bold().Foreground(s.Theme.TextMuted)}
	}
	s.row(b, cols, cells)
}

// row draws one line and trims the trailing blanks, so a golden file carries no
// invisible whitespace to argue about.
func (s Sheet) row(b *strings.Builder, cols []column, cells []cell) {
	var line strings.Builder
	line.WriteString(indent)

	for i, c := range cols {
		if i > 0 {
			line.WriteString(strings.Repeat(" ", theme.SpaceNormal))
		}

		text := ""
		style := lipgloss.NewStyle()
		if i < len(cells) {
			text, style = cells[i].text, cells[i].style
		}
		// Prose reaching a cell is folded for the icon tier before it is
		// measured, so ASCII mode never carries a typographic character and the
		// width the cell reserves is the width that is drawn.
		text = theme.Truncate(s.Theme.Copy(text), c.width, s.Theme.Ellipsis())

		// The padding is written outside the style, never inside it. A styled
		// cell ends in an escape sequence, so padding placed within it cannot
		// be trimmed afterwards, and a golden file full of invisible trailing
		// whitespace is a diff nobody can read.
		gap := c.width - theme.Width(text)
		left := 0
		if c.align == alignCentre {
			left = gap / 2
		}

		line.WriteString(strings.Repeat(" ", left))
		if text != "" {
			line.WriteString(style.Render(text))
		}
		line.WriteString(strings.Repeat(" ", gap-left))
	}

	b.WriteString(strings.TrimRight(line.String(), " "))
	b.WriteString("\n")
}

func (s Sheet) heading(b *strings.Builder, text string) {
	b.WriteString("\n")
	b.WriteString(s.bold().Foreground(s.Theme.TextPrimary).Render(s.Theme.Copy(text)))
	b.WriteString("\n")
}

// wrapped writes prose as one or more indented lines within the sheet's width.
func (s Sheet) wrapped(b *strings.Builder, text string, style lipgloss.Style) {
	limit := s.Width - len(indent)
	for _, line := range strings.Split(wrap(s.Theme.Copy(text), limit), "\n") {
		b.WriteString(indent)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
}

// ---------------------------------------------------------------------------
// Cell contents
// ---------------------------------------------------------------------------

// swatch is a block of the colour itself, so a reader can see what the number
// means. It keeps its width in both icon tiers.
func (s Sheet) swatch() string {
	if s.Theme.Icons == theme.ASCII {
		return "####"
	}
	return "████"
}

// ratio is the token's worst measured contrast, which is the number that
// decides whether it is readable.
func (s Sheet) ratio(r theme.Role) string {
	switch r.Kind {
	case theme.KindSurface, theme.KindDecoration:
		return ""
	}
	worst := 0.0
	for _, p := range s.Theme.Contrast() {
		if p.Foreground == r.Token && (worst == 0 || p.Ratio < worst) {
			worst = p.Ratio
		}
	}
	if worst == 0 {
		return ""
	}
	return theme.FormatRatio(worst)
}

// sample is text painted in the role, so the reader judges the colour on the
// thing it is actually used for rather than on a square.
func (s Sheet) sample(r theme.Role) string {
	switch r.Kind {
	case theme.KindSurface, theme.KindDecoration:
		return ""
	}
	return "The quick tanuki"
}

// fillCount replaces the count a real row would supply, so the phrase reads as
// it will on screen rather than as a format string.
func fillCount(phrase string) string {
	return strings.ReplaceAll(phrase, "%d", "2")
}

func colourFor(th theme.Theme, ok bool) color.Color {
	if ok {
		return th.StatusSuccess
	}
	return th.StatusWarning
}

func (s Sheet) bold() lipgloss.Style { return lipgloss.NewStyle().Bold(true) }

// separator is the loose gap between metadata groups in the header.
func (s Sheet) separator() string {
	if s.Theme.Icons == theme.ASCII {
		return " | "
	}
	return " · "
}

func containsKind(kinds []theme.Kind, k theme.Kind) bool {
	for _, want := range kinds {
		if want == k {
			return true
		}
	}
	return false
}

// wrap breaks text on spaces at limit cells. It is here rather than in the
// theme because wrapping a paragraph is a preview concern; a table cell is
// truncated, never wrapped.
func wrap(text string, limit int) string {
	if limit <= 0 {
		return text
	}
	var lines []string
	var line strings.Builder

	for _, word := range strings.Fields(text) {
		switch {
		case line.Len() == 0:
			line.WriteString(word)
		case theme.Width(line.String())+1+theme.Width(word) <= limit:
			line.WriteString(" ")
			line.WriteString(word)
		default:
			lines = append(lines, line.String())
			line.Reset()
			line.WriteString(word)
		}
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}
