// Package view is the contract between the shell and the four dashboard views.
//
// Views depend on this and never on the root model, which is what stops the
// per-view split turning into a circular import the first time a view needs
// something from the shell.
package view

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/tui/layout"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// Services is what the shell lends a view. The root wires the implementation
// in, so a view can be built in a test with a fake.
type Services interface {
	// Now is the injected clock. Nothing in labdash calls time.Now directly.
	Now() time.Time
}

// An Env is everything a view is handed on each update and each render.
type Env struct {
	Theme    theme.Theme
	Layout   layout.Layout
	Services Services
	// Scope and Offline reach the view because availability is answered from
	// the same context the shell answers it from.
	Scope   action.Scope
	Offline bool
}

// A Section is one tab in the carousel.
type Section struct {
	Name string
	// Count is the number of rows behind the tab. A negative count is not yet
	// known and renders as nothing rather than as zero.
	Count int
}

// Unknown is the count of a section that has not been fetched.
const Unknown = -1

// Do asks a view to perform one action. The shell resolves the keystroke
// against the keymap and checks availability first, so a view never decides
// whether a key applies and never names a key of its own.
type Do struct{ Action keymap.Action }

// A View is one dashboard screen.
type View interface {
	Screen() action.Screen
	Title() string
	Sections() []Section
	Section() int
	Selection() int
	Row() action.Row
	Update(tea.Msg, Env) (View, tea.Cmd)
	// Body renders the rows region: everything between the table header and
	// the footer.
	Body(Env) string
}
