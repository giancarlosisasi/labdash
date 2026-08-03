// Package frame draws the chrome every dashboard screen sits inside: the
// border, the context bar, the section tabs, the table header, the footer and
// the overlay.
//
// Nothing here holds state. Each function takes the layout and returns a string
// of exactly the width it was given, so a resize is a re-render and never a
// repair.
package frame

import (
	"strings"

	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// A Box draws the frame at one width, in one theme.
type Box struct {
	Theme theme.Theme
	Width int
}

// Top and Bottom are the frame's two horizontal edges.
func (b Box) Top() string    { return b.edge(b.borders().TopLeft, b.borders().TopRight) }
func (b Box) Bottom() string { return b.edge(b.borders().BottomLeft, b.borders().BottomRight) }

// Rule is a horizontal rule spanning the frame.
func (b Box) Rule() string { return b.edge(b.borders().TeeLeft, b.borders().TeeRight) }

// SplitRule is a rule with a junction where a pane divider meets it. down draws
// the junction for a divider that starts below the rule; up, for one that ends
// at it.
func (b Box) SplitRule(dividerAt int, down bool) string {
	br := b.borders()
	junction := br.TeeUp
	if down {
		junction = br.TeeDown
	}

	inner := b.Width - 2
	if dividerAt <= 0 || dividerAt >= inner {
		return b.Rule()
	}
	line := strings.Repeat(br.Horizontal, dividerAt) + junction +
		strings.Repeat(br.Horizontal, inner-dividerAt-1)
	return b.paint(br.TeeLeft + line + br.TeeRight)
}

// Row wraps one line of content in the frame, padded to exactly the frame's
// width. Exactly: a line one cell wider than its allowance moves the border,
// and a moving border is the artefact a resize is judged by.
func (b Box) Row(content string) string {
	br := b.borders()
	inner := theme.Pad(content, b.Width-4, b.Theme.Ellipsis())
	return b.paint(br.Vertical) + " " + inner + " " + b.paint(br.Vertical)
}

// SplitRow wraps two panes and the divider between them.
func (b Box) SplitRow(left string, leftWidth int, right string, rightWidth int) string {
	br := b.borders()
	v := b.paint(br.Vertical)
	return v + " " + theme.Pad(left, leftWidth, b.Theme.Ellipsis()) + " " +
		v + " " + theme.Pad(right, rightWidth, b.Theme.Ellipsis()) + " " + v
}

// Divider is the column a split pane is separated by. It is a colour change on
// focus and never a geometry change.
func (b Box) Divider() string { return b.paint(b.borders().Vertical) }

func (b Box) edge(left, right string) string {
	return b.paint(left + strings.Repeat(b.borders().Horizontal, max(0, b.Width-2)) + right)
}

func (b Box) borders() theme.Borders { return b.Theme.Borders() }

func (b Box) paint(s string) string { return b.Theme.Style(b.Theme.BorderDefault).Render(s) }
