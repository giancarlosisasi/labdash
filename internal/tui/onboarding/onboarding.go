// Package onboarding is the first run: screens S-02, S-03 and S-04.
//
// It is a view inside the frame rather than a prompt sequence before the
// program starts, so resize, `?`, theming, crash safety and the non-TTY rule
// all apply to it without a second answer to any of them.
//
// It knows nothing about credentials. Everything it can set in motion goes
// through Service, which keeps internal/tui free of the auth package and lets a
// model test drive the whole flow with no network at all.
package onboarding

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/clock"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// A Code is a device authorization waiting to be approved.
type Code struct {
	// UserCode is shown exactly as the instance wrote it.
	UserCode string
	// URL carries the code in its query string, so opening it fills the form in.
	URL string
	// Expires is when the code dies. GitLab gives five minutes.
	Expires time.Time
}

// An Account is a finished login.
type Account struct {
	Host     string
	Username string
	// Scope is what the credential may do, already converted from the granted
	// scopes. The wizard shows it and never interprets it.
	Scope action.Scope
}

// A Service is the credential work the wizard sets in motion.
//
// Every method that talks to an instance is called from a tea.Cmd and may block
// for as long as it needs; none of them is called from the update loop.
type Service interface {
	// Known reports whether the settings file already has an entry for a host.
	Known(host string) bool
	// StartDevice asks the instance for a one-time code.
	StartDevice(ctx context.Context, host string) (Code, error)
	// AwaitApproval waits for the code started by StartDevice.
	AwaitApproval(ctx context.Context, host string) (Account, error)
	// DeviceUnsupported reports whether a StartDevice failure means the
	// instance is older than GitLab 17.9 rather than that something is wrong.
	DeviceUnsupported(err error) bool
	// LoginWithBrowser runs the loopback flow with PKCE.
	LoginWithBrowser(ctx context.Context, host string) (Account, error)
	// LoginWithToken validates and stores a pasted personal access token.
	LoginWithToken(ctx context.Context, host, token string) (Account, error)
	// SaveInstance writes the host's stanza to the settings file, and nothing
	// else. A credential never reaches that file.
	SaveInstance(host string) error
	// OpenURL hands a URL to the platform's browser.
	OpenURL(url string) error
}

// A step is one screen of the flow. Esc walks back through them, and there is
// no fourth level below stepToken.
type step int

const (
	stepHost step = iota
	stepMethod
	stepDevice
	stepToken
	stepOffer
	stepDone
)

// Options is what the wizard needs to start.
type Options struct {
	Theme   theme.Theme
	Clock   clock.Clock
	Service Service
	// Host is the instance the settings file already prefers, used as the
	// default so a returning user presses Enter twice.
	Host string
	// Version is shown beside the wordmark on the welcome screen.
	Version string
}

// A Model is the wizard.
type Model struct {
	theme   theme.Theme
	clock   clock.Clock
	service Service
	version string

	step step
	// cursor is the selected row of whichever list this step draws.
	cursor int

	host      string
	hostField textinput.Model
	// customHost is true once the user has chosen to name their own instance,
	// which is what reveals the field on S-02.
	customHost bool

	tokenField textinput.Model

	code  Code
	frame int

	account Account
	// failure replaces the step's body with the auth case of S-18. It names what
	// failed and leaves two keys that change the situation.
	failure string
	// busy names the work in flight, lowercase, as the copy formula asks.
	busy string
}

// New builds the wizard at its first screen.
func New(opts Options) *Model {
	c := opts.Clock
	if c == nil {
		c = clock.System()
	}

	host := opts.Host
	if host == "" {
		host = "gitlab.com"
	}

	// fieldWidth is wide enough for a long self-managed hostname and for the
	// masked run of a personal access token. The field never grows, so the
	// screen under it never moves.
	const fieldWidth = 48

	hostField := textinput.New()
	hostField.Placeholder = "gitlab.example.com"
	hostField.Prompt = ""
	hostField.SetWidth(fieldWidth)

	tokenField := textinput.New()
	tokenField.Placeholder = "paste it here"
	tokenField.Prompt = ""
	tokenField.SetWidth(fieldWidth)
	// A pasted credential must not reach the screen, the scrollback, or a
	// golden file.
	tokenField.EchoMode = textinput.EchoPassword

	return &Model{
		theme:      opts.Theme,
		clock:      c,
		service:    opts.Service,
		version:    opts.Version,
		host:       host,
		hostField:  hostField,
		tokenField: tokenField,
	}
}

// Screen is the context every availability question about this model is
// answered in.
func (m *Model) Screen() action.Screen { return action.SignIn }

// Title is what the footer's left region names.
func (m *Model) Title() string { return "Sign in" }

// Done reports the finished login, and whether there is one. The shell reads it
// to know when to swap the wizard for the dashboard.
func (m *Model) Done() (Account, bool) { return m.account, m.step == stepDone }

// Init starts nothing. The welcome screen is a question, and asking an instance
// for a code before the user has chosen one would be work nobody asked for.
func (m *Model) Init() tea.Cmd { return nil }

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

type (
	// codeMsg is a device authorization the user now has to approve.
	codeMsg struct{ code Code }
	// approvedMsg is a completed login, by whichever route.
	approvedMsg struct{ account Account }
	// failedMsg is anything that went wrong, already written for a person.
	failedMsg struct{ reason string }
	// fallbackMsg means the instance is too old for the device flow, so the
	// browser flow takes over without asking the user to know why.
	fallbackMsg struct{}
	// tickMsg advances the spinner and the countdown.
	tickMsg struct{}
)

// spinnerInterval is ten frames a second, which is one revolution a second for
// the ten-frame braille set.
const spinnerInterval = 100 * time.Millisecond

func tick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update handles the wizard's own messages. Keystrokes arrive as Do, already
// resolved against the keymap and checked for availability by the shell, so
// nothing here matches on a literal key except the text fields.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case codeMsg:
		m.code, m.busy, m.step = msg.code, "waiting for approval", stepDevice
		return tea.Batch(tick(), m.awaitApproval(), m.openApproval())

	case approvedMsg:
		m.account, m.busy, m.failure = msg.account, "", ""
		if m.service.Known(m.account.Host) {
			m.step = stepDone
			return nil
		}
		m.step, m.cursor = stepOffer, 0
		return nil

	case fallbackMsg:
		m.busy = "opening your browser"
		return m.browserLogin()

	case failedMsg:
		m.failure, m.busy = msg.reason, ""
		return nil

	case tickMsg:
		if m.busy == "" {
			return nil
		}
		m.frame++
		return tick()

	case tea.KeyPressMsg:
		return m.typing(msg)
	}

	return nil
}

// Do performs one resolved action.
func (m *Model) Do(id keymap.Action) tea.Cmd {
	switch id {
	case keymap.Down:
		m.cursor = min(m.cursor+1, m.choices()-1)
	case keymap.Up:
		m.cursor = max(m.cursor-1, 0)
	case keymap.SignInContinue:
		return m.advance()
	case keymap.SignInUseToken:
		m.failure, m.busy = "", ""
		m.step = stepToken
		return m.tokenField.Focus()
	case keymap.SignInCopyCode:
		if m.code.UserCode != "" {
			return tea.SetClipboard(m.code.UserCode)
		}
	case keymap.SignInOpenApproval:
		return m.openApproval()
	case keymap.Back:
		m.back()
	}
	return nil
}

// typing feeds a keystroke to whichever field is focused. Everything else is a
// bound action and never reaches here.
func (m *Model) typing(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
	switch {
	case m.step == stepToken:
		m.tokenField, cmd = m.tokenField.Update(msg)
	case m.step == stepHost && m.customHost:
		m.hostField, cmd = m.hostField.Update(msg)
	}
	return cmd
}

// Typing reports whether a keystroke belongs to a focused text field rather
// than to the keymap. The shell asks before it resolves a key, so that typing
// "t" into a hostname does not fire use-a-token.
func (m *Model) Typing() bool {
	return m.step == stepToken || (m.step == stepHost && m.customHost)
}

// advance is Enter: it commits whatever the current step was asking for.
func (m *Model) advance() tea.Cmd {
	m.failure = ""

	switch m.step {
	case stepHost:
		if m.cursor == 1 && !m.customHost {
			m.customHost = true
			return m.hostField.Focus()
		}
		if m.customHost {
			named := strings.TrimSpace(m.hostField.Value())
			if named == "" {
				m.failure = "That is not a hostname yet. Type one, or press Esc to go back."
				return nil
			}
			m.host = strings.TrimSuffix(strings.TrimPrefix(named, "https://"), "/")
		} else {
			m.host = "gitlab.com"
		}
		m.hostField.Blur()
		m.step, m.cursor = stepMethod, 0
		return nil

	case stepMethod:
		switch m.cursor {
		case 0:
			m.busy = "asking " + m.host + " for a code"
			return tea.Batch(tick(), m.startDevice())
		case 1:
			m.busy = "opening your browser"
			return tea.Batch(tick(), m.browserLogin())
		default:
			m.step = stepToken
			return m.tokenField.Focus()
		}

	case stepToken:
		token := strings.TrimSpace(m.tokenField.Value())
		if token == "" {
			m.failure = "No token was pasted. Paste one, or press Esc to choose another way in."
			return nil
		}
		m.busy = "checking the token with " + m.host
		return tea.Batch(tick(), m.tokenLogin(token))

	case stepOffer:
		if m.cursor == 0 {
			if err := m.service.SaveInstance(m.account.Host); err != nil {
				m.failure = "The settings file could not be written — " + err.Error() +
					" Press Enter to carry on without saving it."
				m.cursor = 1
				return nil
			}
		}
		m.step = stepDone
		return nil
	}

	return nil
}

// back goes up exactly one level. At the first screen there is nothing above,
// so it stays put: Esc never quits.
func (m *Model) back() {
	m.failure, m.busy = "", ""

	switch m.step {
	case stepHost:
		if m.customHost {
			m.customHost = false
			m.hostField.Blur()
			m.hostField.Reset()
		}
	case stepMethod:
		m.step, m.cursor = stepHost, 0
	case stepDevice:
		m.code = Code{}
		m.step, m.cursor = stepMethod, 0
	case stepToken:
		m.tokenField.Blur()
		m.tokenField.Reset()
		m.step, m.cursor = stepMethod, 0
	case stepOffer:
		// Declining is a first-class outcome: the login already succeeded and
		// nothing about it is undone by not writing the instance down.
		m.step = stepDone
	}
}

// choices is how many rows the current step's list has, so movement cannot run
// off either end.
func (m *Model) choices() int {
	switch m.step {
	case stepHost:
		return 2
	case stepMethod:
		return 3
	case stepOffer:
		return 2
	default:
		return 1
	}
}

// ---------------------------------------------------------------------------
// Commands. Every one of these talks to an instance, so every one of them is a
// tea.Cmd and none of them is called from Update.
// ---------------------------------------------------------------------------

func (m *Model) startDevice() tea.Cmd {
	host, service := m.host, m.service
	return func() tea.Msg {
		code, err := service.StartDevice(context.Background(), host)
		switch {
		case err == nil:
			return codeMsg{code}
		case service.DeviceUnsupported(err):
			return fallbackMsg{}
		default:
			return failedMsg{reason: cannotReach(host, err)}
		}
	}
}

func (m *Model) awaitApproval() tea.Cmd {
	host, service := m.host, m.service
	return func() tea.Msg {
		account, err := service.AwaitApproval(context.Background(), host)
		if err != nil {
			return failedMsg{reason: firstSentence(err) + " Press Esc to start again, or t to use a token."}
		}
		return approvedMsg{account}
	}
}

func (m *Model) browserLogin() tea.Cmd {
	host, service := m.host, m.service
	return func() tea.Msg {
		account, err := service.LoginWithBrowser(context.Background(), host)
		if err != nil {
			return failedMsg{reason: firstSentence(err) + " Press Esc to go back, or t to use a token."}
		}
		return approvedMsg{account}
	}
}

func (m *Model) tokenLogin(token string) tea.Cmd {
	host, service := m.host, m.service
	return func() tea.Msg {
		account, err := service.LoginWithToken(context.Background(), host, token)
		if err != nil {
			return failedMsg{reason: firstSentence(err) +
				" Check the token has the api scope, then press Enter to try again."}
		}
		return approvedMsg{account}
	}
}

// openApproval sends the user to the page that takes the code. A browser that
// will not open is not a failure: the URL is on screen and o tries again.
func (m *Model) openApproval() tea.Cmd {
	url, service := m.code.URL, m.service
	if url == "" {
		return nil
	}
	return func() tea.Msg {
		_ = service.OpenURL(url)
		return nil
	}
}

// cannotReach is the error formula: what failed, why, what to do.
func cannotReach(host string, err error) string {
	return host + " could not be reached — " + firstSentence(err) +
		" Press Esc to choose another instance, or t to use a token."
}

// firstSentence keeps an error to the part a person can act on. A wrapped Go
// error chain is a stack trace with punctuation.
func firstSentence(err error) string {
	if err == nil {
		return ""
	}
	line, _, _ := strings.Cut(err.Error(), "\n")
	if i := strings.Index(line, ": "); i > 0 && i < len(line)-2 {
		line = line[i+2:]
	}
	if line == "" {
		return "the instance did not answer."
	}
	if !strings.HasSuffix(line, ".") {
		line += "."
	}
	return line
}
