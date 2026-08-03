// Package terminal puts a terminal back the way labdash found it.
//
// One restore path, called from the normal quit, from the crash handler, and
// from the suspend hook. Three implementations is how one of them ends up
// missing mouse reporting and a user is left clicking in a dead shell.
package terminal

import (
	"io"
	"strings"
)

// Sequences undoes everything a TUI does to a terminal, in the order that
// leaves the least on screen if the write is cut short.
//
// Each is a no-op against a terminal that was never put into that state, so the
// whole set is emitted whether or not a TUI ran. A terminal left in raw mode
// with a hidden cursor is a shell the user has to close.
func Sequences() []string {
	return []string{
		"\x1b[?1049l", // leave the alternate screen
		"\x1b[?25h",   // show the cursor
		"\x1b[?1000l", // mouse tracking off
		"\x1b[?1002l", // button-event tracking off
		"\x1b[?1003l", // any-event tracking off
		"\x1b[?1006l", // SGR mouse mode off
		"\x1b[?2004l", // bracketed paste off
		"\x1b[?7h",    // line wrap back on
		"\x1b[0m",     // no styling
	}
}

// Restore writes the sequences to w. Errors are dropped: this runs while the
// process is already dying, and there is nowhere left to report to.
func Restore(w io.Writer) {
	_, _ = io.WriteString(w, strings.Join(Sequences(), ""))
}
