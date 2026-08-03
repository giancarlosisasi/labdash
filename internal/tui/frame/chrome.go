package frame

import (
	"fmt"
	"strings"

	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// activeMarker sits in a one-cell slot every tab has, so switching tabs never
// moves the tab beside it.
var activeMarker = theme.Glyph{Unicode: "▍", ASCII: ">", Word: "active"}

var (
	overflowLeft  = theme.Glyph{Unicode: "‹", ASCII: "<", Word: "more to the left"}
	overflowRight = theme.Glyph{Unicode: "›", ASCII: ">", Word: "more to the right"}
	warning       = theme.Glyph{Unicode: "⚠", ASCII: "!", Word: "error"}
)

// A Tab is one entry in either tab row.
type Tab struct {
	Name string
	// Count is rendered beside the name. Negative is not yet known and renders
	// as nothing, which is honest in a way that zero is not.
	Count int
}

// ContextBar is the view tabs on the left and the badges on the right.
//
// The active view is uppercase, which is the only place uppercase is used for
// anything but a column header.
func ContextBar(th theme.Theme, width int, views []Tab, active int, badges []string) string {
	var names []string
	for i, v := range views {
		slot := " "
		style := th.Style(th.TextMuted)
		name := strings.ToLower(v.Name)
		if i == active {
			slot = th.Style(th.AccentPrimary).Render(th.Glyph(activeMarker))
			style = th.Style(th.AccentPrimary).Bold(true)
			name = strings.ToUpper(v.Name)
		}
		names = append(names, slot+style.Render(name))
	}

	left := strings.Join(names, strings.Repeat(" ", theme.SpaceNormal))
	right := th.Style(th.TextMuted).Render(strings.Join(badges, strings.Repeat(" ", theme.SpaceNormal)))
	return spread(th, width, left, right)
}

// SectionTabs is the carousel. Every tab carries a one-cell marker slot and the
// row reserves both arrow slots whether or not they are showing, so nothing on
// this row ever moves sideways because of what is selected.
func SectionTabs(th theme.Theme, width int, tabs []Tab, active int) string {
	if len(tabs) == 0 {
		return ""
	}

	const arrowSlot = 2
	available := width - 2*arrowSlot
	rendered := make([]string, len(tabs))
	widths := make([]int, len(tabs))
	for i, t := range tabs {
		rendered[i] = renderTab(th, t, i == active)
		widths[i] = theme.Width(rendered[i])
	}

	start, end := windowAround(widths, active, available, theme.SpaceNormal)

	row := strings.Join(rendered[start:end], strings.Repeat(" ", theme.SpaceNormal))
	return arrow(th, overflowLeft, start > 0) + row +
		strings.Repeat(" ", max(0, available-theme.Width(row))) +
		arrow(th, overflowRight, end < len(tabs))
}

// windowAround returns the widest run of tabs that fits and contains active,
// preferring to keep the earliest tabs on screen.
func windowAround(widths []int, active, available, gap int) (start, end int) {
	for start = 0; start <= active; start++ {
		used, end := 0, start
		for end < len(widths) {
			next := widths[end]
			if end > start {
				next += gap
			}
			if used+next > available {
				break
			}
			used += next
			end++
		}
		if end > active {
			return start, end
		}
	}
	return active, active + 1
}

func renderTab(th theme.Theme, t Tab, active bool) string {
	slot, style := " ", th.Style(th.TextSecondary)
	if active {
		slot = th.Style(th.AccentPrimary).Render(th.Glyph(activeMarker))
		style = th.Style(th.TextPrimary).Bold(true)
	}

	label := th.Copy(t.Name)
	if t.Count >= 0 {
		label += " " + fmt.Sprint(t.Count)
	}
	return slot + style.Render(label)
}

func arrow(th theme.Theme, g theme.Glyph, show bool) string {
	if !show {
		return "  "
	}
	return th.Style(th.TextMuted).Render(th.Glyph(g)) + " "
}

// A HeaderCell is one column name and the room it has.
type HeaderCell struct {
	Text  string
	Width int
}

// TableHeader is the column names, aligned the way their values are: text left,
// numbers and durations right.
func TableHeader(th theme.Theme, width int, cols []HeaderCell) string {
	var cells []string
	for _, c := range cols {
		cells = append(cells, theme.Pad(c.Text, c.Width, th.Ellipsis()))
	}
	row := strings.Join(cells, strings.Repeat(" ", theme.SpaceNormal))
	return th.Style(th.TextMuted).Render(theme.Pad(row, width, th.Ellipsis()))
}

// A Hint is one key the footer offers, and whether it can be used here.
type Hint struct {
	Key  string
	Word string
	// Available greys the hint rather than removing it. An action nobody can
	// see is an action nobody can ask about.
	Available bool
	// Essential keeps a hint on screen when the row has to shrink. Browse,
	// filter and pin are essential because they are how a view is built at all,
	// and so are help and quit.
	Essential bool
}

// A FooterState is the three regions' content. The footer is always one line:
// a footer that grows shifts the table above it, and a table that shifts breaks
// the spatial memory that is the whole reason to use a terminal.
type FooterState struct {
	// Left names where you are.
	Left string
	// Spinner and Task are the work in flight. Task is lowercase and names what
	// is running.
	Spinner, Task string
	// Errors is how many sections are failing. The footer carries the count;
	// the section itself carries the message.
	Errors int
	// Message is a direct answer to the last keypress, such as a refusal.
	Message string
	Hints   []Hint
}

// Footer renders the three regions, each truncating on its own.
//
// A message is served before the hints. It is the answer to the key that was
// just pressed and it is gone by the next one, where a hint is still there to
// read afterwards. A truncated reason explains nothing, which is the failure
// this ordering exists to prevent.
//
// The hints keep room for their last one whatever the message needs, so the way
// out never disappears.
func Footer(th theme.Theme, width int, s FooterState) string {
	left := th.Style(th.TextSecondary).Render(th.Copy(s.Left))
	leftWidth := min(theme.Width(left), width/3)

	available := max(0, width-leftWidth-2*theme.SpaceLoose)
	hintBudget := available
	if s.Message != "" {
		wanted := theme.Width(th.Copy(s.Message))
		hintBudget = min(max(available-wanted, lastHintWidth(th, s.Hints)), available)
	}

	right := renderHints(th, s.Hints, hintBudget)
	rightWidth := theme.Width(right)

	return strings.Join([]string{
		theme.Pad(left, leftWidth, th.Ellipsis()),
		strings.Repeat(" ", theme.SpaceLoose),
		renderMiddle(th, available-rightWidth, s),
		strings.Repeat(" ", theme.SpaceLoose),
		theme.PadLeft(right, rightWidth, th.Ellipsis()),
	}, "")
}

// renderMiddle keeps the two compact forms whole and spends whatever is left on
// the words.
//
// The spinner says a task is running and the badge says something failed, each
// in one or three cells. On a narrow terminal the sentence beside them is what
// goes, so both states survive down to a footer with almost no middle at all.
func renderMiddle(th theme.Theme, width int, s FooterState) string {
	var head, tail string
	if s.Task != "" && s.Spinner != "" {
		head = th.Style(th.StatusRunning).Render(s.Spinner) + " "
	}
	if s.Errors > 0 {
		tail = strings.Repeat(" ", theme.SpaceNormal) + th.Style(th.StatusError).Render(
			fmt.Sprintf("%s %d", th.Glyph(warning), s.Errors))
	}

	text, style := s.Message, th.Style(th.TextPrimary)
	if text == "" {
		text, style = s.Task, th.Style(th.StatusRunning)
	}

	headWidth, tailWidth := theme.Width(head), theme.Width(tail)
	if headWidth+tailWidth > width {
		// No room for words. The badge outlives the spinner, because a failure
		// is the thing somebody has to act on.
		if tailWidth >= width {
			return theme.Pad(tail, width, th.Ellipsis())
		}
		// No ellipsis on the spinner: one cell of it is the whole message, and
		// an ellipsis would spend that cell saying there is more.
		return theme.Pad(head, width-tailWidth, "") + tail
	}

	room := width - headWidth - tailWidth
	return head + theme.Pad(style.Render(th.Copy(text)), room, th.Ellipsis()) + tail
}

// renderHints greys what cannot run here. An unavailable action is explained,
// never hidden: it stays in place and the reason arrives in the middle region
// when the key is pressed.
//
// When they do not all fit, hints are dropped from the front. The caller orders
// them so that help and quit are last, and those two are the ones a stuck user
// needs on screen.
func renderHints(th theme.Theme, hints []Hint, budget int) string {
	parts := make([]string, len(hints))
	for i, h := range hints {
		key, word := th.Style(th.AccentPrimary), th.Style(th.TextSecondary)
		if !h.Available {
			key, word = th.Style(th.TextMuted), th.Style(th.TextMuted)
		}
		parts[i] = key.Render(h.Key) + " " + word.Render(th.Copy(h.Word))
	}

	gap := strings.Repeat(" ", theme.SpaceNormal)
	kept := make([]int, len(parts))
	for i := range parts {
		kept[i] = i
	}

	// Drop the screen's own verbs from the end first, then the essentials from
	// the front, so the last thing standing is quit.
	for {
		row := join(parts, kept, gap)
		if theme.Width(row) <= budget || len(kept) == 0 {
			return row
		}
		if last := lastOptional(hints, kept); last >= 0 {
			kept = append(kept[:last], kept[last+1:]...)
			continue
		}
		kept = kept[1:]
	}
}

func join(parts []string, kept []int, gap string) string {
	out := make([]string, len(kept))
	for i, k := range kept {
		out[i] = parts[k]
	}
	return strings.Join(out, gap)
}

// lastHintWidth is the room the final hint needs. It is the floor a message
// cannot take, so quit is still on screen when everything else has gone.
func lastHintWidth(th theme.Theme, hints []Hint) int {
	if len(hints) == 0 {
		return 0
	}
	last := hints[len(hints)-1]
	return theme.Width(last.Key) + 1 + theme.Width(th.Copy(last.Word))
}

func lastOptional(hints []Hint, kept []int) int {
	for i := len(kept) - 1; i >= 0; i-- {
		if !hints[kept[i]].Essential {
			return i
		}
	}
	return -1
}

// spread puts left and right at the two ends of one row, and truncates the left
// rather than letting the two collide.
func spread(th theme.Theme, width int, left, right string) string {
	gap := width - theme.Width(left) - theme.Width(right)
	if gap >= theme.SpaceNormal {
		return left + strings.Repeat(" ", gap) + right
	}
	room := max(0, width-theme.Width(right)-theme.SpaceNormal)
	return theme.Pad(left, room, th.Ellipsis()) +
		strings.Repeat(" ", theme.SpaceNormal) + right
}
