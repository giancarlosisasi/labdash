package onboarding

import (
	"fmt"
	"strings"
	"time"

	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/tui/frame"
	"github.com/giancarlosisasi/labdash/internal/tui/layout"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// The compact wordmark, two rows and twenty-six columns, from
// research/17-design-system.md §11.3. It is committed rather than generated so
// there is no font dependency and no startup cost.
var wordmark = []string{
	"█░░ ▄▀█ █▄▄ █▀▄ ▄▀█ █▀ █░█",
	"█▄▄ █▀█ █▄█ █▄▀ █▀█ ▄█ █▀█",
}

// wordmarkASCII is the ACC-04 fallback, figlet's Standard font. It is taller
// and wider than the block form, which is allowed here and nowhere else: the
// welcome screen has no column beside the mark for it to shift.
var wordmarkASCII = []string{
	" _       _         _           _",
	"| | __ _| |__   __| | __ _ ___| |__",
	"| |/ _` | '_ \\ / _` |/ _` / __| '_ \\",
	"| | (_| | |_) | (_| | (_| \\__ \\ | | |",
	"|_|\\__,_|_.__/ \\__,_|\\__,_|___/_| |_|",
}

// minWordmarkWidth is where the mark stops fitting. Below it the screen leads
// with the question instead, which is the part that matters.
const minWordmarkWidth = 40

var (
	selected = theme.Glyph{Unicode: "→", ASCII: ">", Word: "selected"}
	warning  = theme.Glyph{Unicode: "⚠", ASCII: "!", Word: "problem"}
	marker   = theme.Glyph{Unicode: "▍", ASCII: "|", Word: "heading"}
)

// Body renders everything between the frame's top rule and its footer. The
// shell owns the border and the footer, exactly as it does for a dashboard.
func (m *Model) Body(l layout.Layout) []string {
	width := l.Width - 4

	var lines []string
	switch m.step {
	case stepHost:
		lines = m.welcome(width)
	case stepMethod:
		lines = m.method()
	case stepDevice:
		lines = m.device()
	case stepToken:
		lines = m.token()
	case stepOffer:
		lines = m.offer()
	default:
		lines = nil
	}

	if m.failure != "" {
		lines = append(lines, "")
		lines = append(lines, m.problem(width)...)
	}

	return lines
}

// welcome is S-02. It must not read as an error: nothing has gone wrong, the
// application simply does not know which GitLab is yours yet.
func (m *Model) welcome(width int) []string {
	th := m.theme

	lines := []string{""}
	if width >= minWordmarkWidth {
		art := wordmark
		if th.Icons == theme.ASCII {
			art = wordmarkASCII
		}
		for _, row := range art {
			lines = append(lines, th.Style(th.AccentPrimary).Render(row))
		}
		lines = append(lines,
			th.Style(th.TextMuted).Render(th.Copy("every queue that matters   "+m.version)),
			"")
	}

	lines = append(lines,
		th.Style(th.TextPrimary).Render(th.Copy("Let's connect you to a GitLab instance.")),
		"",
		m.heading("Which GitLab?"),
		"",
	)

	lines = append(lines,
		m.choice(0, "gitlab.com", ""),
		m.choice(1, "a self-managed instance"+th.Ellipsis(), ""),
	)

	if m.customHost {
		lines = append(lines, "",
			"    "+th.Style(th.TextSecondary).Render(th.Copy("Hostname"))+"  "+m.hostField.View())
	}

	return lines
}

// method is the first half of S-03: which way in.
func (m *Model) method() []string {
	th := m.theme

	lines := []string{
		"",
		m.heading("Sign in to " + m.host),
		"",
		m.choice(0, "a one-time code", "works over SSH and in containers"),
		m.choice(1, "your browser", "for instances older than GitLab 17.9"),
		m.choice(2, "a personal access token", "no OAuth application needed"),
	}

	if m.busy != "" {
		lines = append(lines, "", "  "+m.spinner()+" "+
			th.Style(th.StatusRunning).Render(th.Copy(m.busy)))
	}

	return lines
}

// device is the second half of S-03: the code, in a box, with a countdown.
//
// The paragraph about GitLab's device form is not filler. GitLab re-renders its
// empty form after you approve, which reads as a failed login, and explaining it
// beforehand costs three lines and prevents a support thread.
func (m *Model) device() []string {
	th := m.theme

	lines := []string{"", m.heading("Sign in to " + m.host), ""}
	lines = append(lines, m.codeBox()...)
	lines = append(lines, "")

	if m.busy != "" {
		lines = append(lines, "  "+m.spinner()+" "+
			th.Style(th.StatusRunning).Render(th.Copy(m.busy)))
	}
	if left := m.remaining(); left != "" {
		lines = append(lines, "    "+th.Style(th.TextMuted).Render(th.Copy("expires in "+left)))
	}

	return append(lines, "",
		th.Style(th.TextSecondary).Render(th.Copy(
			"GitLab redisplays its empty device form after you approve.")),
		th.Style(th.TextSecondary).Render(th.Copy(
			"That is normal — nothing more is needed in the browser.")),
	)
}

// codeBox draws the one-time code inside its own border, because it is the one
// thing on the screen the user has to read character by character.
func (m *Model) codeBox() []string {
	th := m.theme
	br := th.Borders()

	code := th.Style(th.AccentPrimary).Bold(true).Render(m.code.UserCode)
	inner := max(theme.Width(m.code.UserCode)+8, 21)
	rule := strings.Repeat(br.Horizontal, inner)
	edge := th.Style(th.BorderDefault)

	return []string{
		"    " + edge.Render(br.TopLeft+rule+br.TopRight),
		"    " + edge.Render(br.Vertical) + strings.Repeat(" ", inner) + edge.Render(br.Vertical),
		"    " + edge.Render(br.Vertical) + theme.Center(code, inner, th.Ellipsis()) + edge.Render(br.Vertical),
		"    " + edge.Render(br.Vertical) + strings.Repeat(" ", inner) + edge.Render(br.Vertical),
		"    " + edge.Render(br.BottomLeft+rule+br.BottomRight),
		"",
		"    " + th.Style(th.TextPrimary).Render(th.Copy("Enter this code at")),
		"    " + th.Style(th.TextSecondary).Render(th.Copy(m.code.URL)),
	}
}

// token is S-04. Nothing is echoed, and the screen says where to make one.
func (m *Model) token() []string {
	th := m.theme

	return []string{
		"",
		m.heading("Paste a token for " + m.host),
		"",
		"    " + th.Style(th.TextSecondary).Render(th.Copy("Token")) + "  " + m.tokenField.View(),
		"",
		th.Style(th.TextSecondary).Render(th.Copy(
			"Create one at https://" + m.host + "/-/user_settings/personal_access_tokens")),
		th.Style(th.TextSecondary).Render(th.Copy(
			"with the api scope, or read_api to browse without changing anything.")),
		"",
		th.Style(th.TextMuted).Render(th.Copy("Nothing appears as you paste.")),
	}
}

// offer is AUT-13: this host has no settings entry, and transport settings need
// somewhere to live.
func (m *Model) offer() []string {
	th := m.theme

	return []string{
		"",
		m.heading("Signed in to " + m.account.Host),
		"",
		th.Style(th.TextPrimary).Render(th.Copy(
			m.account.Host + " is not in your settings file.")),
		th.Style(th.TextSecondary).Render(th.Copy(
			"Saving it gives caCert, clientCert, proxy and subfolder a home.")),
		"",
		m.choice(0, "Save it", "writes the instance, never the credential"),
		m.choice(1, "Skip", "everything works; there is nowhere to put transport settings"),
	}
}

// problem is the auth case of S-18, drawn under the step's own body. It names
// what failed and leaves keys that change the situation.
//
// It wraps rather than truncates. The end of one of these sentences is the way
// out, and cutting it off leaves the reader with the bad news and nothing else.
func (m *Model) problem(width int) []string {
	th := m.theme
	glyph := th.Style(th.StatusError).Render(th.Glyph(warning)) + " "

	var out []string
	for i, line := range theme.Wrap(th.Copy(m.failure), width-2) {
		prefix := glyph
		if i > 0 {
			prefix = "  "
		}
		out = append(out, prefix+th.Style(th.TextPrimary).Render(line))
	}
	return out
}

// heading is a section heading: the accent bar, then the words.
func (m *Model) heading(text string) string {
	th := m.theme
	return th.Style(th.AccentPrimary).Render(th.Glyph(marker)) + " " +
		th.Style(th.TextPrimary).Bold(true).Render(th.Copy(text))
}

// choice is one row of a list. The marker sits in a slot every row has, so
// moving the selection never shifts the text beside it.
func (m *Model) choice(index int, label, note string) string {
	th := m.theme

	slot, style := " ", th.Style(th.TextSecondary)
	if index == m.cursor {
		slot = th.Style(th.AccentPrimary).Render(th.Glyph(selected))
		style = th.Style(th.TextPrimary).Bold(true)
	}

	text := style.Render(th.Copy(label))
	if note == "" {
		return "   " + slot + "  " + text
	}

	// The notes line up in a column of their own. A ragged second column reads
	// as three unrelated sentences rather than as three choices.
	const labelColumn = 26
	return "   " + slot + "  " + theme.Pad(text, labelColumn, th.Ellipsis()) +
		strings.Repeat(" ", theme.SpaceNormal) + th.Style(th.TextMuted).Render(th.Copy(note))
}

// spinner is the running-work animation, ten frames a second. The word beside
// it carries the state on its own, so switching the motion off loses nothing.
func (m *Model) spinner() string {
	frames := m.theme.SpinnerFrames()
	if len(frames) == 0 {
		return m.theme.ReducedMotionMarker()
	}
	return m.theme.Style(m.theme.StatusRunning).Render(frames[m.frame%len(frames)])
}

// remaining is the countdown on the one-time code, measured against the
// injected clock.
func (m *Model) remaining() string {
	if m.code.Expires.IsZero() {
		return ""
	}
	left := m.code.Expires.Sub(m.clock.Now()).Round(time.Second)
	if left <= 0 {
		return "0:00"
	}
	return fmt.Sprintf("%d:%02d", int(left.Minutes()), int(left.Seconds())%60)
}

// Footer is the keys this step offers, in the order the footer shows them.
//
// They are ids rather than keys: the footer renders them from the same keymap
// everything else reads, so this can never publish a key that does not exist.
func (m *Model) Footer() []keymap.Action {
	switch m.step {
	case stepDevice:
		return []keymap.Action{
			keymap.SignInCopyCode, keymap.SignInOpenApproval, keymap.SignInUseToken, keymap.Back,
		}
	case stepToken:
		return []keymap.Action{keymap.SignInContinue, keymap.Back}
	case stepMethod:
		return []keymap.Action{keymap.SignInContinue, keymap.SignInUseToken, keymap.Back}
	case stepOffer:
		return []keymap.Action{keymap.SignInContinue}
	default:
		return []keymap.Action{keymap.Down, keymap.SignInContinue}
	}
}

// Frame draws the whole screen: the border, the body, and the footer. The
// wizard has no context bar and no tabs, because there is nothing yet to put in
// either and chrome costs rows.
func (m *Model) Frame(l layout.Layout) []string {
	box := frame.Box{Theme: m.theme, Width: l.Width}

	body := m.Body(l)
	// The body region is everything the dashboard spends on tabs, header and
	// rows, so the footer sits exactly where it does on every other screen.
	room := l.SectionTabs + l.TableHeader + l.Body + 1
	for len(body) < room {
		body = append(body, "")
	}
	body = body[:room]

	lines := []string{box.Top()}
	for _, line := range body {
		lines = append(lines, box.Row(line))
	}
	return append(lines, box.Rule())
}
