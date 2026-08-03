package keymap_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/keymap"
)

// KEY-07.T2, and every other coherence rule with it. Prevents: a keymap that
// contradicts itself. With nothing rebindable, a collision is our bug and never
// the user's, and the six rules are the entire reason somebody can guess a key
// they never learned.
func TestKEY07_T2_TheShippedKeymapIsCoherent(t *testing.T) {
	t.Parallel()

	problems := keymap.Check()
	if len(problems) == 0 {
		return
	}

	lines := make([]string, len(problems))
	for i, p := range problems {
		lines[i] = "  " + p.String()
	}
	t.Fatalf("the keymap breaks %d rule(s):\n%s\n\n"+
		"The rules are research/17-design-system.md §6.3.", len(problems), strings.Join(lines, "\n"))
}

// Prevents: a check that passes because it looks at nothing. Each rule is
// pointed at a table that breaks it and watched to fail.
func TestKEY07_T2_TheCheckFiresOnEachPlantedFault(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		rule   string
		break_ func([]keymap.Entry) []keymap.Entry
	}{
		{
			name: "a key bound twice in one context",
			rule: "rule 3",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.Merge, func(e *keymap.Entry) { e.Key = "r" })
			},
		},
		{
			name: "a universal key taken again by one screen",
			rule: "rule 3",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.Approve, func(e *keymap.Entry) { e.Key = "p" })
			},
		},
		{
			name: "one verb reachable by two keys",
			rule: "rule 2",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.RetryJob, func(e *keymap.Entry) { e.Key = "T" })
			},
		},
		{
			name: "a binding with no help text",
			rule: "help text",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.Quit, func(e *keymap.Entry) { e.Help = "" })
			},
		},
		{
			name: "a forbidden key bound",
			rule: "reserved key",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.Pin, func(e *keymap.Entry) { e.Key = "ctrl+s" })
			},
		},
		{
			name: "a digit bound to something other than a chip",
			rule: "reserved key",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.NextView, func(e *keymap.Entry) { e.Key = "1" })
			},
		},
		{
			name: "a key that was left unbound on purpose",
			rule: "unbound key",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.Filter, func(e *keymap.Entry) { e.Key = "e" })
			},
		},
		{
			name: "a timed key sequence",
			rule: "rule 6",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.FirstRow, func(e *keymap.Entry) { e.Key = "g g" })
			},
		},
		{
			name: "an uppercase key that does not escalate its lowercase",
			rule: "rule 1",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.ReopenMR, func(e *keymap.Entry) { e.Key = "T" })
			},
		},
		{
			name: "a vim binding with no arrow equivalent",
			rule: "vim parity",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.Down, func(e *keymap.Entry) { e.Alt = nil })
			},
		},
		{
			name: "search navigation that answers with no search running",
			rule: "rule 4",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.SearchNext, func(e *keymap.Entry) { e.Requires.SearchLive = false })
			},
		},
		{
			name: "n taken by something that is not search navigation",
			rule: "rule 4",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.NewMR, func(e *keymap.Entry) { e.Key = "n" })
			},
		},
		{
			name: "Esc bound to something other than going back",
			rule: "rule 5",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.Quit, func(e *keymap.Entry) { e.Key = "esc" })
			},
		},
		{
			name: "an override that moves a binding instead of replacing it",
			rule: "override",
			break_: func(es []keymap.Entry) []keymap.Entry {
				return set(es, keymap.BrowseExpand, func(e *keymap.Entry) { e.Overrides = keymap.Refresh })
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			problems := keymap.CheckTable(tc.break_(keymap.Table()))
			require.NotEmpty(t, problems, "the planted fault went unreported")

			var rules []string
			for _, p := range problems {
				rules = append(rules, p.Rule)
			}
			require.Contains(t, rules, tc.rule, "reported %v", problems)
		})
	}
}

// KEY-07.T1. Prevents: help text drifting from the keys, which is the failure
// gh-dash carries in both its docs and its code. Nothing that reaches a user
// names a key; every surface reads the table.
func TestKEY07_T1_HelpComesFromTheTable(t *testing.T) {
	t.Parallel()

	for _, e := range keymap.Table() {
		require.NotEmpty(t, e.Help, "%s has no help text", e.ID)
		require.Equal(t, strings.ToLower(e.Help[:1]), e.Help[:1],
			"%s starts its help with a capital; help is a phrase, not a sentence", e.ID)
		require.NotContains(t, e.Help, ".", "%s ends its help in a full stop", e.ID)
	}
}

// KEY-07.T3, restated 2026-08-02. Prevents two failures at once: a binding that
// exists in code and is missing from the cheatsheet, and a finished-tense
// /keys/ page CI would fail against on every release before v1.0.
func TestKEY07_T3_ThePageIsASupersetOfTheOverlay(t *testing.T) {
	t.Parallel()

	page := keymap.Markdown()
	list := keymap.List()

	var overlay []keymap.Action
	for _, screen := range action.Screens() {
		for _, e := range keymap.Implemented(keymap.For(screen)) {
			if !slices.Contains(overlay, e.ID) {
				overlay = append(overlay, e.ID)
			}
		}
	}
	require.NotEmpty(t, overlay, "no action is implemented, so the assertion proves nothing")

	for _, e := range keymap.Table() {
		require.Contains(t, list, string(e.ID)+"\n", "%s is missing from keys --list", e.ID)
		require.Contains(t, page, e.Help, "%s is missing from the published page", e.ID)
	}

	for _, id := range overlay {
		e, ok := keymap.Find(id)
		require.True(t, ok)
		require.Contains(t, page, e.Help, "%s is in the overlay and not on the page", id)
	}
}

// KEY-01 and KEY-02 at the table level: every vim key exists, and every one of
// them has a non-vim way to reach it. Prevents locking out non-vim users, which
// is most of the addressable market.
func TestKEY02_T1_EveryVimKeyHasANonVimEquivalent(t *testing.T) {
	t.Parallel()

	vim := []string{"j", "k", "h", "l", "g", "G", "/", "n", "N", "ctrl+d", "ctrl+u"}
	nonVim := map[string]string{
		"j": "down", "k": "up", "h": "left", "l": "right",
		"g": "home", "G": "end", "ctrl+d": "pgdown", "ctrl+u": "pgup",
	}

	for _, key := range vim {
		e, ok := keymap.Lookup(action.MergeRequests, key)
		require.True(t, ok, "%q is in the L1 tier and is bound to nothing", key)

		want, hasAlternative := nonVim[key]
		if !hasAlternative {
			continue // search and its two navigation keys have no arrow form
		}
		require.Contains(t, e.Keys(), want,
			"%s is reachable by %q and not by %q", e.ID, key, want)
	}
}

// Rule 1 in its positive form. Prevents: the pairs quietly coming apart, which
// is what makes an unlearned uppercase key guessable at all.
func TestKEY07_TheCanonicalCasePairsAreDeclared(t *testing.T) {
	t.Parallel()

	pairs := map[keymap.Action]keymap.Action{
		keymap.ReopenMR:        keymap.CloseMR,
		keymap.ToDoDoneAll:     keymap.ToDoDone,
		keymap.RefreshAll:      keymap.Refresh,
		keymap.ApproveSelected: keymap.Approve,
		keymap.CollapseAll:     keymap.Collapse,
		keymap.CopyURL:         keymap.CopyRef,
		keymap.OpenInEditor:    keymap.OpenInBrowser,
		keymap.SearchPrev:      keymap.SearchNext,
		keymap.Unassign:        keymap.Assign,
		keymap.LastRow:         keymap.FirstRow,
		keymap.LoadMore:        keymap.Down,
	}

	for upper, lower := range pairs {
		e, ok := keymap.Find(upper)
		require.True(t, ok, "%s is not declared", upper)
		require.Equal(t, lower, e.Escalates,
			"%s no longer declares that it escalates or reverses %s", upper, lower)
	}
}

// Prevents: the one context-dependent meaning in the keymap spreading. Browse
// displaces exactly three universal bindings and nothing else does.
//
// Only an override that changes the verb counts. A screen may claim a universal
// key for the same verb — the sign-in screen's `o` still opens a browser — and
// that is not a second meaning to learn, it is the same one narrowed to the
// screen that can act on it.
func TestKEY07_OnlyBrowseOverridesAUniversalBinding(t *testing.T) {
	t.Parallel()

	verbs := map[keymap.Action]keymap.Verb{}
	for _, e := range keymap.Table() {
		verbs[e.ID] = e.Verb
	}

	overriders := map[action.Screen][]keymap.Action{}
	for _, e := range keymap.Table() {
		if e.Overrides != "" && verbs[e.Overrides] != e.Verb {
			overriders[e.Screen] = append(overriders[e.Screen], e.Overrides)
		}
	}

	require.ElementsMatch(t,
		[]keymap.Action{keymap.NextSection, keymap.PrevSection, keymap.FocusPane},
		overriders[action.Browse])

	for screen, overridden := range overriders {
		if screen == action.Browse {
			continue
		}
		require.Subset(t, []keymap.Action{keymap.CopyRef, keymap.FocusPane}, overridden,
			"%s overrides %v; a second context-dependent key is a new thing to learn",
			screen, overridden)
	}
}

// set returns a copy of the table with one entry changed.
func set(entries []keymap.Entry, id keymap.Action, f func(*keymap.Entry)) []keymap.Entry {
	out := slices.Clone(entries)
	for i := range out {
		if out[i].ID == id {
			f(&out[i])
		}
	}
	return out
}
