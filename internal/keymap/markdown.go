package keymap

import (
	"fmt"
	"slices"
	"strings"

	"github.com/giancarlosisasi/labdash/internal/action"
)

// Markdown is the whole keymap as the published /keys/ page.
//
// The page and the command are the same bytes: CI diffs one against the other,
// so a binding cannot change without the cheatsheet changing with it. That is
// also why the whole table is emitted, implemented or not — a page that grew a
// section every release would be a page nobody could pin to a wall.
func Markdown() string {
	var b strings.Builder

	b.WriteString(pageHeader)
	writeTiers(&b)
	writeSection(&b, "Browse, filter, pin", TheThree(), theThreeNote)
	writeSection(&b, "Everywhere", universal(), everywhereNote)

	for _, screen := range action.Screens() {
		own := ownBindings(screen)
		if len(own) == 0 {
			continue
		}
		writeSection(&b, screen.Title(), own, screenNotes[screen])
	}

	b.WriteString(pageFooter)
	return b.String()
}

// List is every declared action, one name per line, for a bug report or a
// proposal to cite. It names the unbuilt actions too, because those are the
// ones somebody is most likely to be asking about.
func List() string {
	names := make([]string, 0, len(table))
	for _, e := range table {
		names = append(names, string(e.ID))
	}
	slices.Sort(names)
	return strings.Join(names, "\n") + "\n"
}

// universal is the bindings that resolve everywhere, minus the three that lead
// the page under their own heading.
func universal() []Entry {
	lead := []Action{Browse, Filter, Pin}
	var out []Entry
	for _, e := range table {
		if e.Screen == action.Everywhere && !slices.Contains(lead, e.ID) {
			out = append(out, e)
		}
	}
	return out
}

func ownBindings(screen action.Screen) []Entry {
	var out []Entry
	for _, e := range table {
		if e.Screen == screen {
			out = append(out, e)
		}
	}
	return out
}

func writeSection(b *strings.Builder, heading string, entries []Entry, note string) {
	fmt.Fprintf(b, "\n## %s\n\n", heading)
	b.WriteString("| Key | Action |\n| --- | --- |\n")
	for _, e := range entries {
		fmt.Fprintf(b, "| %s | %s |\n", code(e.Labels()), e.Help)
	}
	if note != "" {
		fmt.Fprintf(b, "\n%s\n", note)
	}
}

func writeTiers(b *strings.Builder) {
	b.WriteString("\n## The four tiers\n\n")
	b.WriteString("| Tier | Contains | Where you find it |\n| --- | --- | --- |\n")

	where := map[Tier]string{
		L0: "The footer, always",
		L1: "The footer and `?`",
		L2: "`?` and the command palette",
		L3: "This page",
	}
	for _, tier := range Tiers() {
		fmt.Fprintf(b, "| L%d %s | %s | %s |\n",
			int(tier), strings.ToLower(tier.Title()), tierContents(tier), where[tier])
	}

	b.WriteString("\n" + tiersNote + "\n")
}

// tierContents lists a tier's keys, except for L2, where forty mnemonics would
// be a wall of backticks rather than a summary.
func tierContents(tier Tier) string {
	if tier == L2 {
		var n int
		for _, e := range table {
			if e.Tier == L2 {
				n++
			}
		}
		return fmt.Sprintf("%d mnemonics, one for each verb", n)
	}

	var keys []string
	appendKey := func(k string) {
		if l := Label(k); !slices.Contains(keys, l) {
			keys = append(keys, l)
		}
	}

	for _, e := range table {
		switch {
		case e.Tier == tier:
			appendKey(e.Key)
		case tier == L0 && e.Tier == L1:
			// The arrows are L0 by being the non-vim half of an L1 binding, so
			// they belong to no L0 entry's own key.
			for _, k := range e.Alt {
				appendKey(k)
			}
		}
	}
	return code(keys)
}

func code(labels []string) string {
	out := make([]string, len(labels))
	for i, l := range labels {
		out[i] = "`" + l + "`"
	}
	return strings.Join(out, " ")
}

const pageHeader = `---
title: Keyboard reference
description: Every binding labdash has, grouped by where it works, with the rules that make an unlearned key guessable.
---

{/* GENERATED — do not edit. ` + "`labdash keys --markdown`" + ` writes this page and CI fails on a diff. Change internal/keymap instead. */}

# Keyboard reference

Bindings are grouped by where they work, because that is how they are learned. Inside the application, ` + "`?`" + ` shows the same list, narrowed to the screen you are on.

:::info Generated from the keymap
This page is the output of ` + "`labdash keys --markdown`" + `, which reads the table the help overlay reads. The two cannot drift.
:::

Keys are fixed. Every action below has one binding, in every view, on every platform.
`

const tiersNote = `**L0 and L1 are complete alternatives to each other.** Nobody is locked out for not knowing vim, and nobody who knows vim has to reach for an arrow key.`

const theThreeNote = `Learn these three and you can build any view labdash can show. They are L0, so they sit in the footer wherever you are.`

const everywhereNote = `The preview position has no key. ` + "`auto`" + ` puts the pane on the right at 120 columns or wider, and along the bottom below that.`

var screenNotes = map[action.Screen]string{
	action.Browse: "`h` and `l` walk the tree here instead of switching tabs, because Browse has no tabs to switch. " +
		"It is the one context-dependent meaning in the whole keymap, and it is the universal tree convention.",
	action.Trace: "`w` follows a growing log, which is the same verb as watching a running pipeline.",
}

const pageFooter = `
## Coherence rules

Six rules cover the whole keymap. They are what let you guess a key you have not learned.

1. **Lowercase acts on the row; uppercase escalates or reverses it.** ` + "`x`" + ` closes, ` + "`X`" + ` reopens. ` + "`d`" + ` marks one done, ` + "`D`" + ` marks all. ` + "`r`" + ` refreshes one, ` + "`Ctrl+R`" + ` refreshes all.
2. **A key means the same verb in every view.** ` + "`R`" + ` retries. ` + "`x`" + ` stops. ` + "`m`" + ` makes it go: merge a merge request, or play a manual job. ` + "`L`" + ` shows logs. ` + "`w`" + ` keeps showing you a live thing. ` + "`z`" + ` collapses a group, a tree node, or a log section.
3. **A key resolves to one action in the context you are in.** Nothing is bound twice in the same view, so what you learned in one table still holds in the next.
4. **` + "`n`" + ` and ` + "`N`" + ` are search navigation only**, and only while a search is live. That is why "new merge request" is ` + "`Ctrl+N`" + `.
5. **` + "`Esc`" + ` goes back one level and ` + "`q`" + ` quits.** ` + "`Esc`" + ` will not discard a half-typed comment without asking.
6. **Chords are instantaneous.** A chord is ` + "`Ctrl`" + `-prefixed and lands on one keystroke. A ` + "`g`" + `-then-` + "`g`" + ` with a timeout produces mystery input on a slow SSH link.

## Reserved keys

| Key | Why it stays free |
| --- | --- |
| ` + "`Ctrl+S`" + ` | Most terminals deliver it as XOFF and freeze the display. |
| ` + "`Ctrl+I`" + ` | A terminal delivers it as ` + "`Tab`" + `, which is already the view switcher. |
| ` + "`Ctrl+M`" + ` | A terminal delivers it as ` + "`Enter`" + `. |
| ` + "`Ctrl+C`" + ` | Interrupt belongs to the shell. |
| ` + "`e`" + ` ` + "`E`" + ` | Editing a filter is ` + "`f`" + ` and reaching the code is ` + "`O`" + `. Leaving these two free makes displaced muscle memory fail loudly. |
| ` + "`1`" + `–` + "`5`" + ` | Digits belong to the quick-filter chips, so they never select a view or a tab. |

## Markdown export

` + "```bash" + `
labdash keys --markdown > cheatsheet.md
` + "```" + `

The same content, from the same source, for pinning next to your monitor.

` + "```bash" + `
labdash keys --list
` + "```" + `

Every action name, one per line, for a bug report or a proposal to cite.
`
