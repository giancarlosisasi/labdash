package shell

import (
	tea "charm.land/bubbletea/v2"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/tui/view"
)

// handleKey is the only place a keystroke becomes an action.
//
// Nothing below matches on a literal key: the keystroke is resolved against the
// keymap, checked against the one availability predicate, and only then
// dispatched. A key bound to something that cannot run here always produces an
// answer rather than silence.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.message = ""

	if m.overlay != noOverlay {
		m.overlay = noOverlay
		return m, nil
	}

	// A focused text field owns every keystroke that is not a way out of it.
	// Otherwise typing a hostname containing "t" would fire use-a-token.
	if m.wizard != nil && m.wizard.Typing() && !leavesAField(msg.String()) {
		return m, m.runWizard(func() tea.Cmd { return m.wizard.Update(msg) })
	}

	entry, bound := keymap.Lookup(m.context().Screen, msg.String())
	if !bound {
		return m, nil
	}

	if available := entry.Available(m.context()); !available.OK {
		m.message = entry.Refusal(m.context())
		return m, nil
	}

	// Quit and help belong to the shell on every screen, wizard included: they
	// are the two keys a stuck user reaches for and neither depends on where
	// they are.
	switch entry.ID {
	case keymap.Quit:
		return m, tea.Quit
	case keymap.Help:
		m.overlay = helpOverlay
		return m, nil
	}

	if m.wizard != nil {
		return m, m.runWizard(func() tea.Cmd { return m.wizard.Do(entry.ID) })
	}

	switch entry.ID {
	case keymap.SignInAgain:
		if m.signIn == nil {
			return m, nil
		}
		m.wizard = m.signIn(m.failingHost)
		m.scope, m.username = action.ScopeNone, ""
		return m, m.wizard.Init()
	case keymap.Back:
		// Esc goes back exactly one level and never quits. On a dashboard with
		// nothing above it, that is nothing at all.
		return m, nil
	case keymap.NextView:
		m.active = (m.active + 1) % len(m.views)
		return m, nil
	case keymap.PrevView:
		m.active = (m.active - 1 + len(m.views)) % len(m.views)
		return m, nil
	}

	return m, m.routeToView(view.Do{Action: entry.ID})
}

// leavesAField is the two keys a focused text field must not swallow: one
// commits what was typed, the other abandons it. Everything else is text.
func leavesAField(key string) bool { return key == "enter" || key == "esc" }

// footerActions is what the footer offers, in the order it offers them: the
// three keys that replaced the config file, then this screen's own verbs, then
// the two that are always last.
//
// Unavailable actions stay in the list and are greyed. Hiding them would leave
// a fixed keymap with keys nobody can find out about.
func (m *Model) footerActions() []footerAction {
	if m.wizard != nil {
		return m.wizardFooter()
	}

	lead := []keymap.Action{keymap.Browse, keymap.Filter, keymap.Pin}
	tail := []keymap.Action{keymap.Help, keymap.Quit}

	var out []footerAction
	for _, id := range lead {
		if e, ok := keymap.Find(id); ok {
			out = append(out, footerAction{e, true})
		}
	}

	const ownVerbs = 3
	var own int
	for _, e := range keymap.For(m.current().Screen()) {
		if e.Screen == m.current().Screen() && own < ownVerbs {
			out = append(out, footerAction{e, false})
			own++
		}
	}

	for _, id := range tail {
		if e, ok := keymap.Find(id); ok {
			out = append(out, footerAction{e, true})
		}
	}
	return out
}

// wizardFooter is the first-run flow's own hints. Browse, filter and pin are
// absent because there is nothing to browse, filter or pin until there is a
// credential — and the wizard step names what it needs instead.
func (m *Model) wizardFooter() []footerAction {
	var out []footerAction
	for _, id := range m.wizard.Footer() {
		if e, ok := keymap.Find(id); ok {
			out = append(out, footerAction{e, false})
		}
	}
	for _, id := range []keymap.Action{keymap.Help, keymap.Quit} {
		if e, ok := keymap.Find(id); ok {
			out = append(out, footerAction{e, true})
		}
	}
	return out
}

// A footerAction is a binding and whether it survives a narrow footer.
type footerAction struct {
	entry     keymap.Entry
	essential bool
}
