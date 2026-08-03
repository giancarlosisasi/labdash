package keymap

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/giancarlosisasi/labdash/internal/action"
)

// A Problem is one thing wrong with a keymap. It carries the rule it breaks so
// that a failure names the reason and not only the symptom.
type Problem struct {
	// Rule is the coherence rule or the reserved-key rule the entry breaks.
	Rule string
	// Detail is the sentence a contributor reads.
	Detail string
}

func (p Problem) String() string { return p.Rule + ": " + p.Detail }

// forbiddenKeys are never bound, and each is a different way a terminal takes
// the key away from us before we see it.
//
// Ctrl+D is deliberately absent: it is page-down, in the vim tier, exactly as
// every pager and vim itself binds it.
var forbiddenKeys = map[string]string{
	"ctrl+s": "most terminals still deliver Ctrl+S as XOFF and freeze the display",
	"ctrl+i": "a terminal delivers Ctrl+I as Tab, which is already the view switcher",
	"ctrl+m": "a terminal delivers Ctrl+M as Enter",
	"ctrl+c": "Ctrl+C is interrupt, and taking it would break every user's reflex",
}

// unboundKeys stay free so that displaced muscle memory fails loudly rather
// than doing something else.
var unboundKeys = map[string]string{
	"e": "e was the old edit-this-filter key; filtering is f now, and a second key for one verb breaks rule 2",
	"E": "E was reserved for a user-defined shell binding; shell bindings are retired and reaching the code from a row is O",
}

// quickFilterActions are the only actions a digit may be bound to.
var quickFilterActions = []Action{
	QuickFilter1, QuickFilter2, QuickFilter3, QuickFilter4, QuickFilter5,
}

// vimKeys are the L1 set. An L1 binding whose alternates are all vim keys would
// lock out somebody who does not know vim, which is most of the market.
var vimKeys = map[string]bool{
	"j": true, "k": true, "h": true, "l": true, "g": true, "G": true,
	"/": true, "n": true, "N": true, "ctrl+d": true, "ctrl+u": true,
}

// Check reports everything wrong with the shipped keymap. An empty result is
// the only acceptable one, and the build asserts it.
func Check() []Problem { return CheckTable(table) }

// CheckTable runs every rule over an arbitrary table.
//
// It is exported so that the test can plant a collision in a copy and watch the
// check fail first. A static check nobody has seen fire is a static check that
// finds nothing.
func CheckTable(entries []Entry) []Problem {
	var problems []Problem
	add := func(rule, format string, args ...any) {
		problems = append(problems, Problem{rule, fmt.Sprintf(format, args...)})
	}

	checkWellFormed(entries, add)
	checkCollisions(entries, add)
	checkOneVerbOneKey(entries, add)
	checkReserved(entries, add)
	checkEscalations(entries, add)
	checkOverrides(entries, add)
	checkVimParity(entries, add)
	checkSearchNavigation(entries, add)
	checkNavigationKeys(entries, add)

	return problems
}

type reporter func(rule, format string, args ...any)

// checkWellFormed asserts every field a surface reads is populated, so a
// binding with no help text fails the build rather than printing a blank cell.
func checkWellFormed(entries []Entry, add reporter) {
	seen := map[Action]bool{}
	for _, e := range entries {
		switch {
		case e.ID == "":
			add("declaration", "an entry bound to %q has no action id", e.Key)
		case seen[e.ID]:
			add("declaration", "%s is declared twice", e.ID)
		}
		seen[e.ID] = true

		if e.Key == "" {
			add("declaration", "%s has no key", e.ID)
		}
		if e.Help == "" {
			add("help text", "%s has no help text, so `?` and /keys/ would print a blank cell", e.ID)
		}
		if e.Verb == "" {
			add("declaration", "%s has no verb, so a refusal cannot be phrased for it", e.ID)
		}
		if e.Screen == "" {
			add("declaration", "%s names no screen", e.ID)
		}
		for _, k := range e.Keys() {
			if strings.Contains(k, " ") {
				add("rule 6", "%s is bound to the sequence %q; a chord is Ctrl-prefixed "+
					"and instantaneous, and a timed sequence is mystery input on a slow link", e.ID, k)
			}
		}
	}
}

// checkCollisions is the only version of the collision check worth running:
// over the whole table, per screen, with the universal set merged in.
func checkCollisions(entries []Entry, add reporter) {
	for _, screen := range action.Screens() {
		owner := map[string]Action{}
		for _, e := range forScreen(entries, screen) {
			for _, k := range e.Keys() {
				if prior, taken := owner[k]; taken {
					add("rule 3", "%q is bound to both %s and %s on %s",
						k, prior, e.ID, screen)
					continue
				}
				owner[k] = e.ID
			}
		}
	}
}

// checkOneVerbOneKey is coherence rule 2 in its checkable form. The rule governs
// verbs and not letters: `d` is draft-toggle on a merge request and mark-done on
// a To-Do, and that is allowed. What it forbids is one concept reachable by two
// different keys.
func checkOneVerbOneKey(entries []Entry, add reporter) {
	keysFor := map[Verb]string{}
	ownerFor := map[Verb]Action{}
	for _, e := range entries {
		signature := strings.Join(e.Keys(), " ")
		prior, seen := keysFor[e.Verb]
		if !seen {
			keysFor[e.Verb] = signature
			ownerFor[e.Verb] = e.ID
			continue
		}
		if prior != signature {
			add("rule 2", "%q is reachable by %q (%s) and by %q (%s); one verb takes one key "+
				"in every view", e.Verb, prior, ownerFor[e.Verb], signature, e.ID)
		}
	}
}

// checkReserved covers the keys that are never bound and the keys that are
// bound to one feature only.
func checkReserved(entries []Entry, add reporter) {
	for _, e := range entries {
		for _, k := range e.Keys() {
			if why, forbidden := forbiddenKeys[k]; forbidden {
				add("reserved key", "%s is bound to %q — %s", e.ID, k, why)
			}
			if why, unbound := unboundKeys[k]; unbound {
				add("unbound key", "%s is bound to %q — %s", e.ID, k, why)
			}
			if len(k) == 1 && unicode.IsDigit(rune(k[0])) && !slices.Contains(quickFilterActions, e.ID) {
				add("reserved key", "%s is bound to the digit %q; digits are reserved for "+
					"quick-filter chips and are never view or tab selectors", e.ID, k)
			}
		}
	}
}

// checkEscalations is coherence rule 1: lowercase acts on the row, uppercase
// escalates or reverses it. It checks the declared links rather than guessing
// at which pairs ought to exist — `U` is update-branch and `u` is undo, and
// they are deliberately unrelated.
func checkEscalations(entries []Entry, add reporter) {
	byID := map[Action]Entry{}
	for _, e := range entries {
		byID[e.ID] = e
	}

	for _, e := range entries {
		if e.Escalates == "" {
			continue
		}
		target, ok := byID[e.Escalates]
		if !ok {
			add("rule 1", "%s escalates %s, which is not declared", e.ID, e.Escalates)
			continue
		}
		if target.Escalates != "" {
			add("rule 1", "%s escalates %s, which already escalates %s; escalation is one step",
				e.ID, target.ID, target.Escalates)
		}
		if !escalatesKey(e.Key, target.Key) {
			add("rule 1", "%s (%q) escalates %s (%q), but %q is neither the uppercase nor "+
				"the Ctrl form of %q", e.ID, e.Key, target.ID, target.Key, e.Key, target.Key)
		}
		if !reachableTogether(entries, e, target) {
			add("rule 1", "%s escalates %s, but the two never resolve on the same screen",
				e.ID, target.ID)
		}
	}
}

// escalatesKey reports whether upper is the uppercase or the Ctrl form of lower.
// Both shapes are in the design: `x`/`X` closes and reopens, `r`/`Ctrl+R`
// refreshes one and all.
func escalatesKey(upper, lower string) bool {
	if len(lower) != 1 {
		return false
	}
	return upper == strings.ToUpper(lower) || upper == "ctrl+"+lower
}

// reachableTogether reports whether both entries resolve on at least one screen.
func reachableTogether(entries []Entry, a, b Entry) bool {
	for _, screen := range action.Screens() {
		var foundA, foundB bool
		for _, e := range forScreen(entries, screen) {
			foundA = foundA || e.ID == a.ID
			foundB = foundB || e.ID == b.ID
		}
		if foundA && foundB {
			return true
		}
	}
	return false
}

// checkOverrides asserts that a screen only ever displaces a universal binding,
// and only one it actually shadows.
func checkOverrides(entries []Entry, add reporter) {
	byID := map[Action]Entry{}
	for _, e := range entries {
		byID[e.ID] = e
	}

	for _, e := range entries {
		if e.Overrides == "" {
			continue
		}
		target, ok := byID[e.Overrides]
		switch {
		case !ok:
			add("override", "%s overrides %s, which is not declared", e.ID, e.Overrides)
		case e.Screen == action.Everywhere:
			add("override", "%s is universal and cannot override anything", e.ID)
		case target.Screen != action.Everywhere:
			add("override", "%s overrides %s, which is not a universal binding", e.ID, target.ID)
		case !slices.Equal(e.Keys(), target.Keys()):
			add("override", "%s overrides %s but takes %q where %s takes %q; an override "+
				"replaces a binding, it does not move it",
				e.ID, target.ID, e.Render(), target.ID, target.Render())
		}
	}
}

// noArrowEquivalent are the L1 bindings that carry no second key, each with the
// reason. The parity rule is about movement: nothing moves the selection or the
// section without an arrow, and these three move neither.
//
// An exemption with no reason is how a rule stops applying to anything, so the
// reason is required alongside it.
var noArrowEquivalent = map[Action]string{
	Search:     "`/` opens a search in every pager and file manager, so it is not a vim idiom to have parity with",
	SearchNext: "n exists only while a search is live, and a search is reached from the palette as well",
	SearchPrev: "N exists only while a search is live, and a search is reached from the palette as well",
}

// checkVimParity holds the two tiers to being complete alternatives for
// movement: every vim binding that moves something carries a non-vim way to
// reach it.
func checkVimParity(entries []Entry, add reporter) {
	for _, e := range entries {
		if e.Tier != L1 || !vimKeys[e.Key] {
			continue
		}
		if _, exempt := noArrowEquivalent[e.ID]; exempt {
			continue
		}
		if !slices.ContainsFunc(e.Alt, func(k string) bool { return !vimKeys[k] }) {
			add("vim parity", "%s is a vim binding (%q) with no non-vim alternative; nobody is "+
				"locked out for not knowing vim", e.ID, e.Key)
		}
	}
}

// checkSearchNavigation is coherence rule 4: n and N navigate matches, and only
// while a search is live. That is why new-merge-request is Ctrl+N.
func checkSearchNavigation(entries []Entry, add reporter) {
	for _, e := range entries {
		isSearchNav := e.ID == SearchNext || e.ID == SearchPrev
		bindsN := slices.Contains(e.Keys(), "n") || slices.Contains(e.Keys(), "N")

		if bindsN && !isSearchNav {
			add("rule 4", "%s is bound to %q; n and N are search navigation only", e.ID, e.Key)
		}
		if isSearchNav && !e.Requires.SearchLive {
			add("rule 4", "%s does not require a live search, so it would answer when "+
				"there is nothing to navigate", e.ID)
		}
	}
}

// checkNavigationKeys is coherence rule 5: Esc never quits and q never goes
// back.
//
// The rule is asserted structurally rather than behaviourally. Whatever screen
// you are on, those two keys resolve to the two actions named here and to
// nothing else, so no screen can quietly make Esc mean "leave".
func checkNavigationKeys(entries []Entry, add reporter) {
	for _, screen := range action.Screens() {
		for _, e := range forScreen(entries, screen) {
			if slices.Contains(e.Keys(), "esc") && e.ID != Back {
				add("rule 5", "esc is bound to %s on %s; Esc goes back one level and "+
					"never quits", e.ID, screen)
			}
			if slices.Contains(e.Keys(), "q") && e.ID != Quit {
				add("rule 5", "q is bound to %s on %s; q quits and never goes back",
					e.ID, screen)
			}
		}
	}
}
