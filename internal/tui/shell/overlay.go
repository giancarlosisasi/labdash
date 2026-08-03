package shell

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// withOverlay draws the overlay centred on top of the frame.
//
// The frame underneath is drawn in full and then covered, rather than being
// rebuilt at a smaller size. That keeps the frame's geometry identical whether
// an overlay is open or not, so closing one puts every column back exactly
// where it was.
func (m *Model) withOverlay(frameLines, overlay []string) []string {
	if len(overlay) == 0 {
		return frameLines
	}

	left := max(0, (m.layout.Width-m.layout.OverlayWidth)/2)
	top := max(0, (len(frameLines)-len(overlay))/2)

	out := make([]string, len(frameLines))
	copy(out, frameLines)

	for i, line := range overlay {
		row := top + i
		if row < 0 || row >= len(out) {
			continue
		}
		out[row] = splice(out[row], line, left, m.layout.Width)
	}
	return out
}

// splice puts over on top of under, starting at column at, and returns a line
// of exactly width cells.
//
// The cuts are ANSI-aware: measuring or slicing a styled line with len() is how
// a frame gains a stray escape sequence and every column after it shifts.
func splice(under, over string, at, width int) string {
	head := ansi.Truncate(under, at, "")
	head += pad(at - theme.Width(head))

	tailStart := at + theme.Width(over)
	tail := ansi.TruncateLeft(under, tailStart, "")

	line := head + over + tail
	if got := theme.Width(line); got > width {
		return ansi.Truncate(line, width, "")
	} else if got < width {
		return line + pad(width-got)
	}
	return line
}

func pad(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = ' '
	}
	return string(out)
}
