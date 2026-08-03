// Package help draws S-13, the `?` overlay.
//
// It holds no strings of its own beyond its own chrome: every binding, every
// word and every grouping comes from the keymap, which is what keeps help from
// drifting away from the keys.
package help

import (
	"fmt"
	"strings"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/tui/frame"
	"github.com/giancarlosisasi/labdash/internal/tui/layout"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// Title is the overlay's heading, and what a test looks for.
const Title = "Help"

// Bindings is what the overlay lists: the actions this build does, on this
// screen, that can run right now.
//
// The three that replaced the config file lead, because a user who learns only
// browse, filter and pin can build any view labdash can show.
func Bindings(ctx action.Context) []keymap.Entry {
	available := keymap.Available(keymap.Implemented(keymap.For(ctx.Screen)), ctx)

	lead := map[keymap.Action]bool{keymap.Browse: true, keymap.Filter: true, keymap.Pin: true}
	var first, rest []keymap.Entry
	for _, e := range available {
		if lead[e.ID] {
			first = append(first, e)
			continue
		}
		rest = append(rest, e)
	}
	return append(first, rest...)
}

// Render returns the overlay's lines, already bordered and exactly
// l.OverlayWidth cells wide.
func Render(th theme.Theme, l layout.Layout, ctx action.Context) []string {
	box := frame.Box{Theme: th, Width: l.OverlayWidth}
	inner := l.OverlayWidth - 4

	body := content(th, inner, ctx)
	if room := l.Height - 5; room > 0 && len(body) > room {
		body = append(body[:room], th.Style(th.TextMuted).Render(
			th.Copy(fmt.Sprintf("… %d more, and all of them on /keys/", len(body)-room))))
	}

	lines := []string{titled(th, l.OverlayWidth)}
	for _, line := range body {
		lines = append(lines, box.Row(line))
	}
	return append(lines, box.Rule(), box.Row(closeHint(th)), box.Bottom())
}

// content is the list itself: tier headings, then one row per binding.
func content(th theme.Theme, width int, ctx action.Context) []string {
	groups := keymap.ByTier(Bindings(ctx))

	keyColumn := 0
	for _, group := range groups {
		for _, e := range group {
			keyColumn = max(keyColumn, theme.Width(keys(th, e)))
		}
	}

	var lines []string
	for _, group := range groups {
		lines = append(lines, "",
			th.Style(th.TextMuted).Render(strings.ToUpper(group[0].Tier.Title())))
		for _, e := range group {
			lines = append(lines, fmt.Sprintf("  %s  %s",
				theme.Pad(th.Style(th.AccentPrimary).Render(keys(th, e)), keyColumn, th.Ellipsis()),
				theme.Pad(th.Style(th.TextPrimary).Render(th.Copy(e.Help)),
					max(0, width-keyColumn-4), th.Ellipsis())))
		}
	}
	return append(lines, "")
}

// keys renders a binding and its alternates, in the icon tier the terminal can
// draw. Both tiers are one cell per arrow, so the key column is the same width
// either way.
func keys(th theme.Theme, e keymap.Entry) string {
	labels := e.Labels()
	if th.Icons == theme.ASCII {
		labels = e.LabelsASCII()
	}
	return strings.Join(labels, " ")
}

func titled(th theme.Theme, width int) string {
	br := th.Borders()
	heading := " " + Title + " "
	fill := max(0, width-3-theme.Width(heading))
	return th.Style(th.BorderDefault).Render(
		br.TopLeft + br.Horizontal + heading + strings.Repeat(br.Horizontal, fill) + br.TopRight)
}

func closeHint(th theme.Theme) string {
	esc, _ := keymap.Find(keymap.Back)
	label := keymap.Label(esc.Key)
	if th.Icons == theme.ASCII {
		label = keymap.LabelASCII(esc.Key)
	}

	return th.Style(th.AccentPrimary).Render(label) + "  " +
		th.Style(th.TextSecondary).Render(th.Copy("close")) +
		strings.Repeat(" ", theme.SpaceLoose) +
		th.Style(th.TextMuted).Render(th.Copy("labdash keys --markdown lists every key, built or not"))
}
