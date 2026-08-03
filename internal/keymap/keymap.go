// Package keymap is labdash's whole keymap, declared once as data.
//
// Every action the product will ever have is in one table — implemented or not
// — with its key, its tier, its help text, the screen it belongs to, and an
// availability predicate. The `?` overlay, the footer, `labdash keys --list`,
// `labdash keys --markdown` and the published /keys/ page are all views over
// this table, so they cannot drift apart.
//
// Declaring the unimplemented actions is the load-bearing part. A collision
// check over a third of the keymap checks nothing, and a cheatsheet that grows
// a section every release is a cheatsheet nobody can pin to a wall.
//
// Nothing here is rebindable. That is a decision rather than an omission, and
// it is why the coherence rules are asserted rather than merely written down:
// with no escape hatch, an incoherent keymap is a bug everybody gets at once.
package keymap

import (
	"slices"
	"strings"

	"github.com/giancarlosisasi/labdash/internal/action"
)

// An Action is the stable identifier of one thing labdash can do. It is what a
// bug report, a proposal or the command palette cites, so it never changes once
// published.
type Action string

// A Tier groups keys by how they are learned. L0 and L1 are complete
// alternatives to one another: the product is fully usable without pressing a
// vim key, and fully usable without reaching for an arrow key.
type Tier int

const (
	// L0 is the universal set, always present in the footer.
	L0 Tier = iota
	// L1 is the vim set. Every binding in it carries a non-vim alternate.
	L1
	// L2 is one mnemonic key per verb.
	L2
	// L3 is chords: Ctrl-prefixed and instantaneous, never timed sequences.
	L3
)

// Title is the heading a tier appears under in `?` and on /keys/.
func (t Tier) Title() string {
	switch t {
	case L0:
		return "Universal"
	case L1:
		return "Movement"
	case L2:
		return "Actions"
	default:
		return "Chords"
	}
}

// Name is the short form used in a golden file and in a table cell.
func (t Tier) Name() string {
	switch t {
	case L0:
		return "L0"
	case L1:
		return "L1"
	case L2:
		return "L2"
	default:
		return "L3"
	}
}

// Tiers is every tier, in learning order.
func Tiers() []Tier { return []Tier{L0, L1, L2, L3} }

// A Verb is the concept a key carries, independent of what it acts on.
//
// Coherence rule 2 governs verbs, not letters: `d` is draft-toggle on a merge
// request and mark-done on a To-Do, and that is allowed, because each is one
// concept under one key. What the rule forbids is the reverse — one concept
// reachable by two different keys — and that is exactly what a Verb makes
// checkable.
type Verb string

// An Entry is one binding.
type Entry struct {
	ID Action
	// Key is written the way a terminal reports it: "j", "G", "ctrl+r",
	// "shift+tab", "enter", "esc", "pgdown".
	Key string
	// Alt is every equivalent key. For an L1 binding this is where the non-vim
	// alternative lives, and the coherence check asserts it is never empty.
	Alt  []string
	Tier Tier
	Verb Verb
	// Help is the phrase `?` and /keys/ both print.
	Help string
	// Short is the one or two words the footer has room for. Empty falls back
	// to the action's id, which already reads as a verb phrase for most of
	// them.
	Short  string
	Screen action.Screen
	// Overrides names the universal action this binding displaces. Browse is
	// the only place it is used: `h` and `l` walk the tree there because Browse
	// has no section tabs.
	Overrides Action
	// Escalates names the row action this uppercase or Ctrl binding escalates
	// or reverses — coherence rule 1, made checkable.
	Escalates Action
	// Implemented separates `?` and the footer, which show only what this build
	// does, from /keys/, which shows everything.
	Implemented bool
	Requires    action.Requirement
}

func (e Entry) Keys() []string {
	out := make([]string, 0, 1+len(e.Alt))
	out = append(out, e.Key)
	out = append(out, e.Alt...)
	return out
}

// Available is the one predicate for this action; no surface decides for
// itself.
func (e Entry) Available(ctx action.Context) action.Availability {
	req := e.Requires
	req.Built = e.Implemented
	if len(req.Screens) == 0 && e.Screen != action.Everywhere {
		// Where a binding resolves is already the answer to "is it offered
		// here", so no entry restates it.
		req.Screens = []action.Screen{e.Screen}
	}
	return action.Require(req)(ctx)
}

// Refusal is empty when the action is available, and otherwise the sentence
// every surface shows.
func (e Entry) Refusal(ctx action.Context) string {
	return action.Sentence(string(e.Verb), e.Available(ctx))
}

// Table is the whole declared keymap, in the order /keys/ prints it. It copies,
// so no caller can edit the keymap the rest of the product reads.
func Table() []Entry { return slices.Clone(table) }

func Find(id Action) (Entry, bool) {
	for _, e := range table {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// For returns every binding that resolves on a screen: the universal set, minus
// anything the screen overrides, plus the screen's own.
//
// Merging is what makes the collision check real. A key free in the
// merge-requests table but taken universally is still taken.
func For(screen action.Screen) []Entry { return forScreen(table, screen) }

// forScreen is For over an arbitrary table, so that the coherence check can be
// pointed at a deliberately broken one and watched to fail.
func forScreen(entries []Entry, screen action.Screen) []Entry {
	overridden := map[Action]bool{}
	for _, e := range entries {
		if e.Screen == screen && e.Overrides != "" {
			overridden[e.Overrides] = true
		}
	}

	var out []Entry
	for _, e := range entries {
		switch {
		case e.Screen == action.Everywhere && !overridden[e.ID]:
			out = append(out, e)
		case e.Screen == screen:
			out = append(out, e)
		}
	}
	return out
}

// Implemented is what separates `?` from /keys/.
func Implemented(entries []Entry) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.Implemented {
			out = append(out, e)
		}
	}
	return out
}

// Available narrows entries to the ones that can run right now, so `?` never
// offers an action this screen would refuse.
func Available(entries []Entry, ctx action.Context) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.Available(ctx).OK {
			out = append(out, e)
		}
	}
	return out
}

// ByTier groups entries into the four tiers, in learning order. Empty tiers are
// dropped, so a screen with no chords grows no empty heading.
func ByTier(entries []Entry) [][]Entry {
	var out [][]Entry
	for _, tier := range Tiers() {
		var group []Entry
		for _, e := range entries {
			if e.Tier == tier {
				group = append(group, e)
			}
		}
		if len(group) > 0 {
			out = append(out, group)
		}
	}
	return out
}

// Lookup resolves a keystroke to the entry it fires, implemented or not. A
// fixed keymap cannot afford a silent keypress, so the caller always has an
// entry to answer with.
func Lookup(screen action.Screen, keystroke string) (Entry, bool) {
	for _, e := range For(screen) {
		if slices.Contains(e.Keys(), keystroke) {
			return e, true
		}
	}
	return Entry{}, false
}

// TheThree is browse, filter and pin — the keys that replaced the config file.
// They lead the help overlay and the published page because a user who learns
// only these can build any view labdash can show.
func TheThree() []Entry {
	out := make([]Entry, 0, 3)
	for _, id := range []Action{Browse, Filter, Pin} {
		if e, ok := Find(id); ok {
			out = append(out, e)
		}
	}
	return out
}

// Render writes a binding the way the footer and the overlay show it.
func (e Entry) Render() string { return strings.Join(e.Keys(), " ") }

// Word is what the footer prints beside the key.
func (e Entry) Word() string {
	if e.Short != "" {
		return e.Short
	}
	return strings.ReplaceAll(string(e.ID), "-", " ")
}
