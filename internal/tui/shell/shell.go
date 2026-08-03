// Package shell is labdash's root model: the terminal size, the active view,
// the overlay stack and the message routing.
//
// It holds no view state. Each view owns its own model, in its own package, and
// reaches the shell only through the small interface in internal/tui/view — a
// single model with a per-view state struct is how a terminal application grows
// one file nobody can read.
package shell

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/clock"
	"github.com/giancarlosisasi/labdash/internal/tui/onboarding"

	"github.com/giancarlosisasi/labdash/internal/tui/layout"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
	"github.com/giancarlosisasi/labdash/internal/tui/view"
	"github.com/giancarlosisasi/labdash/internal/tui/views/issues"
	"github.com/giancarlosisasi/labdash/internal/tui/views/mr"
	"github.com/giancarlosisasi/labdash/internal/tui/views/pipelines"
	"github.com/giancarlosisasi/labdash/internal/tui/views/todos"
)

// An overlay is what is drawn on top of the dashboard. Only one is ever open:
// an overlay is modal, and modal is expensive.
type overlay int

const (
	noOverlay overlay = iota
	helpOverlay
)

// Options is what the shell needs to start.
type Options struct {
	Theme theme.Theme
	// Clock is what a relative timestamp is measured against. Nothing calls
	// time.Now directly.
	Clock clock.Clock
	// Scope is what the credential may do. It decides, through the one
	// availability predicate, which actions the footer offers.
	Scope action.Scope
	// Wizard is the first-run flow. A missing credential opens it instead of
	// the dashboard; nil means there is already a credential to work with.
	Wizard *onboarding.Model
	// SignIn builds the flow again for an instance whose credential stopped
	// working. It is what Ctrl+A calls, so an expired token is one keypress
	// from being replaced rather than a reason to quit and read a manual.
	SignIn func(host string) *onboarding.Model
	// Verify checks the stored credential once, after the first paint. It is a
	// command, not a call: a dashboard that waited for the network before
	// drawing would break the one rule everything else here follows.
	Verify tea.Cmd
	// Width and Height seed the layout before the terminal reports its size.
	Width, Height int
}

// A Model is the whole application.
type Model struct {
	theme  theme.Theme
	layout layout.Layout
	clock  clock.Clock
	scope  action.Scope

	// signIn rebuilds the wizard for a named instance. See Options.SignIn.
	signIn func(host string) *onboarding.Model
	// verify is the one-off credential check. See Options.Verify.
	verify tea.Cmd

	// wizard replaces the dashboard until there is a credential. It is not one
	// of the views: Tab never reaches it, and it disappears for good once the
	// login completes.
	wizard *onboarding.Model

	views  []view.View
	active int

	overlay overlay
	// message is the answer to the last keypress, cleared by the next one. It
	// is how a key bound to something unavailable stops being silent.
	message string
	offline bool
	// username is who the credential belongs to, shown in the context bar so
	// that a dashboard is visibly yours.
	username string
	// failingHost is the instance whose credential was last refused. Ctrl+A
	// re-runs the login for that one rather than asking the user to re-derive
	// what the application already knows.
	failingHost string
}

// AuthFailed records that an instance refused the credential, so the recovery
// key lands on the right host. The message itself is the caller's, already
// written for a person.
func (m *Model) AuthFailed(host, message string) {
	m.failingHost, m.message = host, message
}

func New(opts Options) *Model {
	c := opts.Clock
	if c == nil {
		c = clock.System()
	}

	return &Model{
		theme:  opts.Theme,
		layout: layout.Compute(opts.Width, opts.Height),
		clock:  c,
		scope:  opts.Scope,
		wizard: opts.Wizard,
		signIn: opts.SignIn,
		verify: opts.Verify,
		views: []view.View{
			mr.New(), pipelines.New(), todos.New(), issues.New(),
		},
	}
}

func (m *Model) Init() tea.Cmd {
	if m.wizard != nil {
		return m.wizard.Init()
	}
	return m.verify
}

// CredentialAccepted names who the stored credential belongs to. It arrives
// after the first paint, which is why the context bar is drawn without it and
// then gains it.
type CredentialAccepted struct{ Username string }

// CredentialRefused is an instance turning the stored credential away. Message
// is already written for a person; Host is what the recovery key re-runs the
// login for.
type CredentialRefused struct{ Host, Message string }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout = layout.Compute(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case CredentialAccepted:
		m.username = msg.Username
		return m, nil

	case CredentialRefused:
		m.AuthFailed(msg.Host, msg.Message)
		return m, nil
	}

	if m.wizard != nil {
		return m, m.runWizard(func() tea.Cmd { return m.wizard.Update(msg) })
	}

	return m, m.routeToView(msg)
}

// runWizard performs one wizard step and adopts its result the moment the login
// finishes. The wizard is dropped rather than hidden: there is no way back to a
// screen whose only purpose has been served.
func (m *Model) runWizard(step func() tea.Cmd) tea.Cmd {
	cmd := step()

	if account, done := m.wizard.Done(); done {
		m.scope = account.Scope
		m.username = account.Username
		m.wizard = nil
	}

	return cmd
}

func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

// Env is what a view is handed. It is rebuilt on each call rather than stored,
// so a view can never read a layout the shell has moved on from.
func (m *Model) env() view.Env {
	return view.Env{
		Theme:    m.theme,
		Layout:   m.layout,
		Services: services{clock: m.clock},
		Scope:    m.scope,
		Offline:  m.offline,
	}
}

// context is what every availability question is answered against, and it is
// built in one place so the footer, the overlay and the key handler cannot
// disagree about where the user is.
func (m *Model) context() action.Context {
	if m.wizard != nil {
		return action.Context{Screen: m.wizard.Screen(), Offline: m.offline}
	}

	v := m.current()
	return action.Context{
		Screen:  v.Screen(),
		Row:     v.Row(),
		Scope:   m.scope,
		Offline: m.offline,
	}
}

// Active is the view the user is looking at.
func (m *Model) Active() view.View { return m.views[m.active] }

func (m *Model) current() view.View { return m.Active() }

func (m *Model) routeToView(msg tea.Msg) tea.Cmd {
	updated, cmd := m.current().Update(msg, m.env())
	m.views[m.active] = updated
	return cmd
}

// services is the shell's side of what a view may ask for.
type services struct{ clock clock.Clock }

func (s services) Now() time.Time { return s.clock.Now() }
