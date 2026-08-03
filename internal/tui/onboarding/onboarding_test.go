package onboarding_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
	"github.com/giancarlosisasi/labdash/internal/testsupport/termcap"
	"github.com/giancarlosisasi/labdash/internal/tui/layout"
	"github.com/giancarlosisasi/labdash/internal/tui/onboarding"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

func TestMain(m *testing.M) { harness.Main(m) }

// A fakeService is the credential layer, scripted. Every route in and every way
// they fail is a field, so a test names the situation rather than building one.
type fakeService struct {
	known map[string]bool

	startErr      error
	unsupported   bool
	approvalErr   error
	browserErr    error
	tokenErr      error
	saveErr       error
	savedInstance string
	openedURL     string
	tokenSeen     string
	account       onboarding.Account
}

func newFakeService() *fakeService {
	return &fakeService{
		known:   map[string]bool{"gitlab.com": true},
		account: onboarding.Account{Host: "gitlab.com", Username: "tanuki", Scope: action.ScopeWrite},
	}
}

func (f *fakeService) Known(host string) bool { return f.known[host] }

func (f *fakeService) StartDevice(_ context.Context, host string) (onboarding.Code, error) {
	if f.startErr != nil {
		return onboarding.Code{}, f.startErr
	}
	return onboarding.Code{
		UserCode: "K7QW-3FDB",
		URL:      "https://" + host + "/-/device?user_code=K7QW-3FDB",
		Expires:  time.Date(2026, 8, 3, 12, 5, 0, 0, time.UTC),
	}, nil
}

func (f *fakeService) AwaitApproval(_ context.Context, host string) (onboarding.Account, error) {
	if f.approvalErr != nil {
		return onboarding.Account{}, f.approvalErr
	}
	return f.accountFor(host), nil
}

func (f *fakeService) DeviceUnsupported(error) bool { return f.unsupported }

func (f *fakeService) LoginWithBrowser(_ context.Context, host string) (onboarding.Account, error) {
	if f.browserErr != nil {
		return onboarding.Account{}, f.browserErr
	}
	return f.accountFor(host), nil
}

func (f *fakeService) LoginWithToken(_ context.Context, host, token string) (onboarding.Account, error) {
	f.tokenSeen = token
	if f.tokenErr != nil {
		return onboarding.Account{}, f.tokenErr
	}
	return f.accountFor(host), nil
}

func (f *fakeService) SaveInstance(host string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.savedInstance = host
	return nil
}

func (f *fakeService) OpenURL(url string) error {
	f.openedURL = url
	return nil
}

func (f *fakeService) accountFor(host string) onboarding.Account {
	acct := f.account
	acct.Host = host
	return acct
}

// wizard builds a model and the layout it draws at.
func wizard(t *testing.T, service onboarding.Service) (*onboarding.Model, layout.Layout) {
	t.Helper()

	th, err := theme.New(theme.Options{Env: termcap.TrueColor, Icons: theme.IconsASCII})
	require.NoError(t, err)

	return onboarding.New(onboarding.Options{
		Theme: th, Service: service, Version: "v0.1.0",
	}), layout.Compute(120, 32)
}

// pump runs a command to completion and feeds every message it produces back
// into the model, which is what the Bubble Tea runtime does. Ticks are dropped:
// a spinner that never stops would make this loop forever.
func pump(t *testing.T, m *onboarding.Model, cmd tea.Cmd) {
	t.Helper()

	for depth := 0; cmd != nil && depth < 20; depth++ {
		msg := cmd()
		switch typed := msg.(type) {
		case nil:
			return
		case tea.BatchMsg:
			cmd = nil
			for _, sub := range typed {
				pump(t, m, sub)
			}
		default:
			if isTick(msg) {
				return
			}
			cmd = m.Update(msg)
		}
	}
}

// isTick recognises the spinner's own message, which the package keeps
// unexported. Draining it would spin this loop for as long as the animation
// runs, which is forever.
func isTick(msg tea.Msg) bool {
	t := reflect.TypeOf(msg)
	return t != nil && strings.Contains(strings.ToLower(t.Name()), "tick")
}

// screen is what the wizard draws, with the colour stripped and the wrapping
// flattened. A sentence that wraps is still one sentence to the reader, and a
// test that could not see across a line break would force the copy to be short
// rather than clear.
func screen(m *onboarding.Model, l layout.Layout) string {
	return strings.Join(strings.Fields(ansi.Strip(strings.Join(m.Body(l), " "))), " ")
}

// AUT-12.T1 — launch with no credential and finish the flow.
//
// Prevents: a first-run experience that is an error message and a manual page
// reference. Nothing on screen is a Go error, and completing the wizard reaches
// a signed-in state.
func TestAUT12T1_TheWizardWalksFromNothingToSignedIn(t *testing.T) {
	t.Parallel()

	service := newFakeService()
	m, l := wizard(t, service)

	welcome := screen(m, l)
	require.Contains(t, welcome, "Which GitLab?")
	require.Contains(t, welcome, "gitlab.com")
	require.Contains(t, welcome, "every queue that matters")
	requireNoGoError(t, welcome)

	pump(t, m, m.Do(keymap.SignInContinue)) // gitlab.com
	require.Contains(t, screen(m, l), "Sign in to gitlab.com")

	pump(t, m, m.Do(keymap.SignInContinue)) // a one-time code

	account, done := m.Done()
	require.True(t, done, "the wizard did not finish; a known host needs no save offer")
	require.Equal(t, "tanuki", account.Username)
	require.Equal(t, action.ScopeWrite, account.Scope)
	require.Equal(t,
		"https://gitlab.com/-/device?user_code=K7QW-3FDB", service.openedURL,
		"the approval page should open itself; every manual step is one that can be mistyped")
}

// AUT-12.T1, the device screen — the code, the countdown and the warning about
// GitLab's own form are all on screen before the poll finishes.
func TestAUT12T1_TheDeviceScreenShowsTheCodeAndWhatComesNext(t *testing.T) {
	t.Parallel()

	service := newFakeService()
	service.approvalErr = errors.New("waiting forever")

	m, l := wizard(t, service)
	pump(t, m, m.Do(keymap.SignInContinue))
	pump(t, m, m.Do(keymap.SignInContinue))

	body := screen(m, l)
	require.Contains(t, body, "K7QW-3FDB", "the one-time code is the whole point of the screen")
	require.Contains(t, body, "redisplays its empty device form",
		"GitLab re-renders its form after approval and that reads as a failure")
	require.Contains(t, body, "Enter this code at")
}

// AUT-12.T2 — leaving the wizard.
//
// Esc goes back exactly one level and never quits, on this screen as on every
// other. At the first screen there is nothing above it, so it stays put; q is
// what leaves, and cmd/labdash prints the instruction on the way out.
func TestAUT12T2_EscGoesBackOneLevelAndNeverTraps(t *testing.T) {
	t.Parallel()

	service := newFakeService()
	service.approvalErr = errors.New("still waiting")

	m, l := wizard(t, service)
	pump(t, m, m.Do(keymap.SignInContinue))
	pump(t, m, m.Do(keymap.SignInContinue))
	require.Contains(t, screen(m, l), "K7QW-3FDB")

	m.Do(keymap.Back)
	require.Contains(t, screen(m, l), "a one-time code", "Esc should return to the method list")

	m.Do(keymap.Back)
	require.Contains(t, screen(m, l), "Which GitLab?", "Esc should return to the welcome screen")

	// The first screen has nothing above it. Esc must not quit, and must not
	// leave the model in a state that draws nothing.
	m.Do(keymap.Back)
	require.Contains(t, screen(m, l), "Which GitLab?")

	_, done := m.Done()
	require.False(t, done, "walking back out of the wizard must not report a login")
}

// 4.5 — an instance too old for the device flow.
//
// Prevents: asking the user to know what a device authorization grant is. The
// fallback happens on its own.
func TestTheDeviceFlowFallsBackToTheBrowserWithoutAsking(t *testing.T) {
	t.Parallel()

	service := newFakeService()
	service.startErr = errors.New("unsupported_grant_type")
	service.unsupported = true

	m, _ := wizard(t, service)
	pump(t, m, m.Do(keymap.SignInContinue))
	pump(t, m, m.Do(keymap.SignInContinue))

	_, done := m.Done()
	require.True(t, done, "the browser flow should have taken over and completed the login")
}

// AUT-13.T1 — logging in to a host with no settings entry.
//
// Prevents: losing caCert and proxy with nowhere to put them. Declining is a
// first-class outcome and still completes the login.
func TestAUT13T1_AnUnknownHostIsOfferedAndDecliningStillSignsIn(t *testing.T) {
	t.Parallel()

	t.Run("accepting writes the instance", func(t *testing.T) {
		t.Parallel()

		service := newFakeService()
		m, l := wizard(t, service)
		signInToUnknownHost(t, m)

		body := screen(m, l)
		require.Contains(t, body, "is not in your settings file")
		require.Contains(t, body, "Save it")
		require.Contains(t, body, "never the credential",
			"the offer must say what it will and will not write")

		pump(t, m, m.Do(keymap.SignInContinue))

		require.Equal(t, "gitlab.example.com", service.savedInstance)
		_, done := m.Done()
		require.True(t, done)
	})

	t.Run("declining still completes the login", func(t *testing.T) {
		t.Parallel()

		service := newFakeService()
		m, _ := wizard(t, service)
		signInToUnknownHost(t, m)

		m.Do(keymap.Down)
		pump(t, m, m.Do(keymap.SignInContinue)) // Skip

		require.Empty(t, service.savedInstance)
		account, done := m.Done()
		require.True(t, done, "declining must not abandon a login that already succeeded")
		require.Equal(t, "gitlab.example.com", account.Host)
	})

	t.Run("a known host is never asked about", func(t *testing.T) {
		t.Parallel()

		service := newFakeService()
		m, _ := wizard(t, service)
		pump(t, m, m.Do(keymap.SignInContinue))
		pump(t, m, m.Do(keymap.SignInContinue))

		_, done := m.Done()
		require.True(t, done, "gitlab.com is already in the settings file")
		require.Empty(t, service.savedInstance)
	})
}

// A pasted token is never echoed and never reaches the screen.
//
// Prevents: a credential in a screenshot, a golden file or a scrollback.
func TestThePastedTokenNeverReachesTheScreen(t *testing.T) {
	t.Parallel()

	const secret = "glpat-never-render-this"

	service := newFakeService()
	m, l := wizard(t, service)

	pump(t, m, m.Do(keymap.SignInContinue))
	pump(t, m, m.Do(keymap.SignInUseToken))
	typeInto(t, m, secret)

	require.NotContains(t, screen(m, l), secret, "the token was drawn on screen")
	require.Contains(t, screen(m, l), "Nothing appears as you paste")

	pump(t, m, m.Do(keymap.SignInContinue))
	require.Equal(t, secret, service.tokenSeen, "the token never reached the credential layer")
}

// A failure names what went wrong and leaves two keys that change it.
//
// Prevents: a raw Go error on the first screen a new user ever sees.
func TestAFailureIsWrittenForAPersonAndOffersAWayOut(t *testing.T) {
	t.Parallel()

	service := newFakeService()
	service.startErr = errors.New("starting device authorization: dial tcp: connection refused")

	m, l := wizard(t, service)
	pump(t, m, m.Do(keymap.SignInContinue))
	pump(t, m, m.Do(keymap.SignInContinue))

	body := screen(m, l)
	require.Contains(t, body, "could not be reached")
	require.Contains(t, body, "Press Esc")
	require.Contains(t, body, "t to use a token")
	requireNoGoError(t, body)
}

// signInToUnknownHost walks the flow to a host that has no settings entry,
// leaving the model on the save offer.
func signInToUnknownHost(t *testing.T, m *onboarding.Model) {
	t.Helper()

	m.Do(keymap.Down)
	pump(t, m, m.Do(keymap.SignInContinue)) // a self-managed instance
	typeInto(t, m, "gitlab.example.com")
	pump(t, m, m.Do(keymap.SignInContinue)) // the hostname
	pump(t, m, m.Do(keymap.SignInContinue)) // a one-time code
}

func typeInto(t *testing.T, m *onboarding.Model, text string) {
	t.Helper()

	require.True(t, m.Typing(), "no field is focused, so this text would fire actions instead")
	for _, r := range text {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// requireNoGoError is the whole of AUT-12.T1's promise about wording: a screen
// in this flow never shows something only a Go programmer can read.
func requireNoGoError(t *testing.T, body string) {
	t.Helper()

	for _, tell := range []string{"panic:", "goroutine ", "0x", "*errors.", "runtime."} {
		require.NotContains(t, body, tell, "a Go error reached the screen")
	}
}
