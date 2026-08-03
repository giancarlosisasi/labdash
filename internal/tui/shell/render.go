package shell

import (
	"fmt"
	"strings"

	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/tui/frame"
	"github.com/giancarlosisasi/labdash/internal/tui/help"
	"github.com/giancarlosisasi/labdash/internal/tui/layout"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

func (m *Model) render() string {
	if m.layout.TooNarrow {
		return m.tooNarrow()
	}

	lines := m.frameLines()
	if m.overlay == helpOverlay {
		lines = m.withOverlay(lines, help.Render(m.theme, m.layout, m.context()))
	}
	return strings.Join(lines, "\n")
}

// frameLines draws the whole screen, top border to bottom border. Every row is
// exactly the terminal's width, so a resize repaints rather than repairs.
func (m *Model) frameLines() []string {
	l := m.layout
	box := frame.Box{Theme: m.theme, Width: l.Width}
	split := l.Preview == layout.RightPreview

	lines := []string{box.Top(), box.Row(m.contextBar())}

	if l.SectionTabs == 1 {
		lines = append(lines, box.Rule(), box.Row(m.sectionTabs()))
	} else {
		lines = append(lines, box.Rule())
	}

	divider := l.TableWidth + 3
	if split {
		lines = append(lines, box.SplitRule(divider, true))
	} else {
		lines = append(lines, box.Rule())
	}

	for i, row := range m.bodyRows() {
		if !split {
			lines = append(lines, box.Row(row))
			continue
		}
		lines = append(lines, box.SplitRow(row, l.TableWidth, m.previewRow(i), l.PreviewWidth))
	}

	if split {
		lines = append(lines, box.SplitRule(divider, false))
	} else {
		lines = append(lines, box.Rule())
	}

	return append(lines, box.Row(m.footer()), box.Bottom())
}

// bodyRows is the table header and the rows beneath it, padded to the height
// the layout allotted. A short body would move the footer, and the footer never
// moves.
func (m *Model) bodyRows() []string {
	l := m.layout
	rows := []string{frame.TableHeader(m.theme, l.TableWidth, headerCells(l))}

	body := m.current().Body(m.env())
	if body != "" {
		rows = append(rows, strings.Split(body, "\n")...)
	}

	for len(rows) < l.TableHeader+l.Body {
		rows = append(rows, "")
	}
	return rows[:l.TableHeader+l.Body]
}

// previewRow is the pane beside the table. It carries its heading and nothing
// else until the detail pane is built.
func (m *Model) previewRow(i int) string {
	th := m.theme
	switch i {
	case 0:
		return th.Style(th.TextMuted).Render(strings.ToUpper("preview"))
	case 2:
		return th.Style(th.TextSecondary).Render(
			th.Copy("Select a row to see its detail here."))
	default:
		return ""
	}
}

func (m *Model) contextBar() string {
	tabs := make([]frame.Tab, len(m.views))
	for i, v := range m.views {
		tabs[i] = frame.Tab{Name: v.Title(), Count: -1}
	}

	var badges []string
	if m.scope == "" {
		badges = append(badges, m.theme.Copy("not signed in"))
	}
	if m.offline {
		badges = append(badges, m.theme.Copy("offline"))
	}

	return frame.ContextBar(m.theme, m.layout.Width-4, tabs, m.active, badges)
}

func (m *Model) sectionTabs() string {
	sections := m.current().Sections()
	tabs := make([]frame.Tab, len(sections))
	for i, s := range sections {
		tabs[i] = frame.Tab{Name: s.Name, Count: s.Count}
	}
	return frame.SectionTabs(m.theme, m.layout.Width-4, tabs, m.current().Section())
}

func (m *Model) footer() string {
	ctx := m.context()

	var hints []frame.Hint
	for _, a := range m.footerActions() {
		key := keymap.Label(a.entry.Key)
		if m.theme.Icons == theme.ASCII {
			key = keymap.LabelASCII(a.entry.Key)
		}
		hints = append(hints, frame.Hint{
			Key:       key,
			Word:      a.entry.Word(),
			Available: a.entry.Available(ctx).OK,
			Essential: a.essential,
		})
	}

	return frame.Footer(m.theme, m.layout.Width-4, frame.FooterState{
		Left:    m.current().Title(),
		Message: m.message,
		Hints:   hints,
	})
}

func headerCells(l layout.Layout) []frame.HeaderCell {
	widths := layout.Widths(l, theme.SpaceNormal)

	var cells []frame.HeaderCell
	for _, c := range layout.ColumnsAt(l.Width) {
		cells = append(cells, frame.HeaderCell{Text: c.Header, Width: widths[c.Name]})
	}
	return cells
}

// tooNarrow names the width the terminal has and the width a dashboard needs.
// A notice that only says "too narrow" leaves the reader guessing at how much
// to drag.
func (m *Model) tooNarrow() string {
	th := m.theme
	lines := []string{
		th.Style(th.StatusWarning).Render(th.Glyph(theme.Glyph{Unicode: "⚠", ASCII: "!", Word: "narrow"})),
		"",
		th.Style(th.TextPrimary).Render(th.Copy(fmt.Sprintf(
			"%d columns is too narrow.", m.layout.Width))),
		th.Style(th.TextPrimary).Render(th.Copy(fmt.Sprintf(
			"labdash needs %d.", layout.MinWidth))),
		"",
		th.Style(th.TextSecondary).Render(th.Copy("Widen the window,")),
		th.Style(th.TextSecondary).Render(th.Copy("or press q to quit.")),
	}

	var out []string
	for range max(0, (m.layout.Height-len(lines))/2) {
		out = append(out, "")
	}
	for _, line := range lines {
		out = append(out, theme.Center(line, m.layout.Width, th.Ellipsis()))
	}
	return strings.Join(out, "\n")
}
