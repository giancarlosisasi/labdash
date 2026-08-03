package view

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// A Base is the movement and section state every view has. Each view embeds one
// and keeps its own identity, rows and empty state.
type Base struct {
	screen    action.Screen
	title     string
	rowKind   action.RowKind
	sections  []Section
	section   int
	selection int
	rows      int
	empty     Empty
}

// An Empty is the copy a view shows when its query returns nothing: what was
// asked for, and the way out.
//
// The way out is never "read the docs". With no config file, changing the
// filter and going somewhere else are the two escapes, so both are offered.
type Empty struct {
	Headline string
	Query    string
}

// NewBase builds the shared state. rowKind is what the view's rows are, which
// is what availability reads to answer "this row cannot take it".
func NewBase(screen action.Screen, title string, rowKind action.RowKind, sections []Section, empty Empty) Base {
	return Base{screen: screen, title: title, rowKind: rowKind, sections: sections, empty: empty}
}

func (b Base) Screen() action.Screen { return b.screen }
func (b Base) Title() string         { return b.title }
func (b Base) Sections() []Section   { return b.sections }
func (b Base) Section() int          { return b.section }
func (b Base) Selection() int        { return b.selection }

// Row is what availability is answered against. An empty table has no row, so
// an action that needs one refuses with "nothing is selected" rather than
// running against a row that is not there.
func (b Base) Row() action.Row {
	if b.rows == 0 {
		return action.Row{}
	}
	return action.Row{Kind: b.rowKind}
}

// Handle performs the movement actions. A view that has nothing of its own to
// do with a message returns it unhandled and the shell moves on.
func (b Base) Handle(msg tea.Msg, env Env) (Base, bool) {
	do, ok := msg.(Do)
	if !ok {
		return b, false
	}

	page := max(1, env.Layout.TableHeight/env.Layout.RowHeight-1)

	switch do.Action {
	case keymap.Down:
		b.selection = b.clamp(b.selection + 1)
	case keymap.Up:
		b.selection = b.clamp(b.selection - 1)
	case keymap.PageDown:
		b.selection = b.clamp(b.selection + page)
	case keymap.PageUp:
		b.selection = b.clamp(b.selection - page)
	case keymap.FirstRow:
		b.selection = 0
	case keymap.LastRow:
		b.selection = b.clamp(b.rows - 1)
	case keymap.NextSection:
		b.section, b.selection = b.wrap(b.section+1), 0
	case keymap.PrevSection:
		b.section, b.selection = b.wrap(b.section-1), 0
	default:
		return b, false
	}
	return b, true
}

func (b Base) clamp(i int) int {
	if b.rows == 0 {
		return 0
	}
	return min(max(i, 0), b.rows-1)
}

func (b Base) wrap(i int) int {
	n := len(b.sections)
	if n == 0 {
		return 0
	}
	return (i%n + n) % n
}

// Body renders the rows region. With no rows it is the empty state, which names
// what was asked for and offers two keys.
func (b Base) Body(env Env) string {
	if b.rows > 0 {
		return ""
	}
	return b.emptyState(env)
}

func (b Base) emptyState(env Env) string {
	th := env.Theme
	width := env.Layout.TableWidth
	height := env.Layout.TableHeight

	lines := []string{
		th.Style(th.TextMuted).Render(th.Glyph(theme.Glyph{Unicode: "⌗", ASCII: "#", Word: "empty"})),
		"",
		th.Style(th.TextPrimary).Render(th.Copy(b.empty.Headline)),
		"",
		th.Style(th.TextSecondary).Render(th.Copy(b.empty.Query)),
		"",
		keyHints(env, keymap.Filter, keymap.Browse, keymap.Help),
	}

	var out strings.Builder
	top := max(0, (height-len(lines))/2)
	for range top {
		out.WriteString("\n")
	}
	for i, line := range lines {
		if i > 0 {
			out.WriteString("\n")
		}
		out.WriteString(theme.Center(line, width, th.Ellipsis()))
	}
	return out.String()
}

// keyHints renders a row of key caps and their words, read from the keymap so
// the empty state cannot offer a key that does not exist.
func keyHints(env Env, ids ...keymap.Action) string {
	th := env.Theme

	var parts []string
	for _, id := range ids {
		e, ok := keymap.Find(id)
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s  %s",
			th.Style(th.AccentPrimary).Render(label(e, th)), th.Style(th.TextMuted).Render(e.Word())))
	}
	return strings.Join(parts, strings.Repeat(" ", theme.SpaceLoose))
}

func label(e keymap.Entry, th theme.Theme) string {
	if th.Icons == theme.ASCII {
		return keymap.LabelASCII(e.Key)
	}
	return keymap.Label(e.Key)
}
