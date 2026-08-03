package shell_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/testsupport/golden"
	"github.com/giancarlosisasi/labdash/internal/testsupport/termcap"
	"github.com/giancarlosisasi/labdash/internal/tui/help"
	"github.com/giancarlosisasi/labdash/internal/tui/onboarding"
	"github.com/giancarlosisasi/labdash/internal/tui/shell"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// scriptedSignIn is a credential layer that always succeeds, so a shell test
// can watch the hand-off from wizard to dashboard without a network.
type scriptedSignIn struct {
	scope action.Scope
	// stall holds the flow on the device screen, which is where the code, the
	// countdown and the four keys are drawn.
	stall bool
}

func (s scriptedSignIn) Known(string) bool { return true }

func (s scriptedSignIn) StartDevice(context.Context, string) (onboarding.Code, error) {
	return onboarding.Code{
		UserCode: "K7QW-3FDB",
		URL:      "https://gitlab.com/-/device?user_code=K7QW-3FDB",
		Expires:  time.Date(2026, 8, 3, 12, 5, 0, 0, time.UTC),
	}, nil
}

func (s scriptedSignIn) AwaitApproval(ctx context.Context, host string) (onboarding.Account, error) {
	if s.stall {
		<-ctx.Done()
	}
	return onboarding.Account{Host: host, Username: "tanuki", Scope: s.scope}, nil
}

func (s scriptedSignIn) DeviceUnsupported(error) bool { return false }

func (s scriptedSignIn) LoginWithBrowser(_ context.Context, host string) (onboarding.Account, error) {
	return onboarding.Account{Host: host, Username: "tanuki", Scope: s.scope}, nil
}

func (s scriptedSignIn) LoginWithToken(_ context.Context, host, _ string) (onboarding.Account, error) {
	return onboarding.Account{Host: host, Username: "tanuki", Scope: s.scope}, nil
}

func (s scriptedSignIn) SaveInstance(string) error { return nil }
func (s scriptedSignIn) OpenURL(string) error      { return nil }

func signInShell(t *testing.T, service onboarding.Service, icons theme.IconSetting) *shell.Model {
	t.Helper()

	th, err := theme.New(theme.Options{Env: termcap.TrueColor, Icons: icons})
	require.NoError(t, err)

	return shell.New(shell.Options{
		Theme: th, Scope: action.ScopeNone, Width: 120, Height: 32,
		Wizard: onboarding.New(onboarding.Options{
			Theme: th, Service: service, Version: "v0.1.0",
		}),
		SignIn: func(string) *onboarding.Model {
			return onboarding.New(onboarding.Options{Theme: th, Service: service})
		},
	})
}

// pressAndSettle sends one key and drains whatever it set in motion, the way
// the runtime does. Ticks are dropped: a spinner never stops.
func pressAndSettle(t *testing.T, m *shell.Model, key string) {
	t.Helper()

	_, cmd := m.Update(keyPress(key))
	drain(t, m, cmd)
}

func drain(t *testing.T, m *shell.Model, cmd tea.Cmd) {
	t.Helper()

	for depth := 0; cmd != nil && depth < 20; depth++ {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				drain(t, m, sub)
			}
			return
		}
		if msg == nil || strings.Contains(strings.ToLower(typeOf(msg)), "tick") {
			return
		}
		_, cmd = m.Update(msg)
	}
}

func typeOf(msg tea.Msg) string { return strings.TrimLeft(fmt.Sprintf("%T", msg), "*") }

func plain(m *shell.Model) string {
	return ansi.Strip(m.View().Content)
}

// AUT-12.T1 — launching with no credential.
//
// Prevents: a first-run experience that is an error message and a manual page.
// The wizard is inside the frame, and finishing it leaves the dashboard chrome
// with the user's name in the context bar.
func TestAUT12T1_TheWizardRendersInsideTheFrameAndHandsOverToTheDashboard(t *testing.T) {
	t.Parallel()

	m := signInShell(t, scriptedSignIn{scope: action.ScopeWrite}, theme.IconsUnicode)

	first := plain(m)
	require.Contains(t, first, "Which GitLab?")
	require.Contains(t, first, "Sign in", "the footer should name where you are")
	require.NotContains(t, first, "MERGE REQUESTS",
		"the dashboard's tabs have nothing to show before there is a credential")

	pressAndSettle(t, m, "enter") // gitlab.com
	pressAndSettle(t, m, "enter") // a one-time code, which the script approves at once

	after := plain(m)
	require.Contains(t, after, "MERGE REQUESTS", "the dashboard chrome should have taken over")
	require.Contains(t, after, "@tanuki", "the context bar should name who you are")
	require.NotContains(t, after, "not signed in")
	require.NotContains(t, after, "Which GitLab?")
}

// AUT-12.T2 — leaving the wizard.
//
// Esc goes back exactly one level and never quits; q quits, from here as from
// anywhere. Neither ever panics or leaves a screen that draws nothing.
func TestAUT12T2_EscNeverQuitsTheWizardAndQAlwaysDoes(t *testing.T) {
	t.Parallel()

	m := signInShell(t, scriptedSignIn{scope: action.ScopeWrite}, theme.IconsUnicode)

	_, cmd := m.Update(keyPress("esc"))
	require.Nil(t, cmd, "Esc quit from the first screen of the wizard")
	require.Contains(t, plain(m), "Which GitLab?", "Esc left the wizard with nothing to draw")

	pressAndSettle(t, m, "enter")
	_, cmd = m.Update(keyPress("esc"))
	require.Nil(t, cmd)
	require.Contains(t, plain(m), "Which GitLab?", "Esc should have gone back exactly one level")

	_, cmd = m.Update(keyPress("q"))
	require.NotNil(t, cmd, "q did not quit")
	require.IsType(t, tea.QuitMsg{}, cmd())
}

// `?` works in the wizard, and lists only what this screen can do.
//
// Prevents: the one key a stuck user reaches for being the one that does
// nothing on the screen they are stuck on.
func TestTheHelpOverlayOpensInTheWizard(t *testing.T) {
	t.Parallel()

	m := signInShell(t, scriptedSignIn{scope: action.ScopeWrite}, theme.IconsUnicode)
	pressAndSettle(t, m, "?")

	body := plain(m)
	require.Contains(t, body, help.Title)
	require.Contains(t, body, "paste a personal access token instead")
	require.NotContains(t, body, "approve it", "a merge request key has no meaning here")
}

// AUT-07.T1 — every mutating action with a read-only credential.
//
// Prevents: offering an action that is guaranteed to 403. The refusal names the
// token's scope, and no mutating binding is listed in the help overlay.
func TestAUT07T1_ReadOnlyDisablesEveryMutatingBinding(t *testing.T) {
	t.Parallel()

	var mutating int
	for _, screen := range action.Screens() {
		ctx := action.Context{Screen: screen, Scope: action.ScopeRead, Row: rowFor(screen)}

		for _, e := range keymap.For(screen) {
			if !e.Requires.Write {
				continue
			}
			mutating++

			require.False(t, e.Available(ctx).OK,
				"%s is available with a read_api token", e.ID)
		}

		for _, e := range help.Bindings(ctx) {
			require.False(t, e.Requires.Write,
				"%s is listed in the help overlay with a read_api token", e.ID)
		}
	}

	require.Positive(t, mutating, "no mutating binding was checked, so this asserted nothing")
}

// AUT-07.T1, the reason — read-only refuses in the token's own words.
//
// It is asserted on the requirement rather than on a binding, because the
// nearest true reason for a key wins: an action this build does not have yet
// says so, and only a built one gets as far as the scope. This is the answer
// every mutating action will give the moment it exists.
func TestAUT07T1_TheRefusalNamesTheScope(t *testing.T) {
	t.Parallel()

	predicate := action.Require(action.Requirement{Built: true, Write: true})
	available := predicate(action.Context{Screen: action.MergeRequests, Scope: action.ScopeRead})

	require.False(t, available.OK)
	require.Contains(t, available.Reason, "read-only")
	require.Equal(t, "Cannot merge — this token is read-only, so labdash cannot change anything.",
		action.Sentence("merge", available))
}

// AUT-07.T1, at the keyboard — a mutating key answers rather than doing
// nothing, and sends no request.
func TestAUT07T1_PressingAMutatingKeyAnswers(t *testing.T) {
	t.Parallel()

	th, err := theme.New(theme.Options{Env: termcap.TrueColor, Icons: theme.IconsUnicode})
	require.NoError(t, err)

	m := shell.New(shell.Options{
		Theme: th, Scope: action.ScopeRead, Width: 120, Height: 32,
	})

	_, cmd := m.Update(keyPress("m"))
	require.Nil(t, cmd, "a refused action must send nothing")
	require.Contains(t, plain(m), "Cannot merge —",
		"a bound key that cannot run here must never be silent")
}

// AUT-07.T2 — the badge.
//
// Prevents: a user not knowing why m does nothing. The mode is stated in a word
// in the context bar, not implied by an absence.
func TestAUT07T2_TheReadOnlyBadgeIsInTheContextBar(t *testing.T) {
	t.Parallel()

	th, err := theme.New(theme.Options{Env: termcap.TrueColor, Icons: theme.IconsUnicode})
	require.NoError(t, err)

	m := shell.New(shell.Options{
		Theme: th, Scope: action.ScopeRead, Width: 120, Height: 32,
	})

	require.Contains(t, plain(m), "read-only")
	golden.Assert(t, "dashboard.state.read-only", m.View().Content+"\n")
}

// Ctrl+A reopens the wizard for the instance that stopped working.
//
// Prevents: an expired token being a reason to quit, read a manual page and run
// a command. It is one keypress, and it lands on the right host.
func TestSignInAgainReopensTheWizard(t *testing.T) {
	t.Parallel()

	th, err := theme.New(theme.Options{Env: termcap.TrueColor, Icons: theme.IconsUnicode})
	require.NoError(t, err)

	var askedFor string
	m := shell.New(shell.Options{
		Theme: th, Scope: action.ScopeWrite, Width: 120, Height: 32,
		SignIn: func(host string) *onboarding.Model {
			askedFor = host
			return onboarding.New(onboarding.Options{
				Theme: th, Service: scriptedSignIn{scope: action.ScopeWrite},
			})
		},
	})

	m.AuthFailed("gitlab.example.com", "gitlab.example.com rejected the credential")
	require.Contains(t, plain(m), "rejected the credential")

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	drain(t, m, cmd)

	require.Equal(t, "gitlab.example.com", askedFor,
		"the recovery must re-run the login for the instance that failed")
	require.Contains(t, plain(m), "Which GitLab?")
}

// The three wizard screens, drawn. A diff here is a screen changing.
func TestGoldenTheWizardScreens(t *testing.T) {
	t.Parallel()

	for _, icons := range []theme.IconSetting{theme.IconsUnicode, theme.IconsASCII} {
		name := string(icons)

		m := signInShell(t, scriptedSignIn{scope: action.ScopeWrite, stall: true}, icons)
		golden.Assert(t, "signin.welcome."+name, m.View().Content+"\n")

		pressAndSettle(t, m, "enter")
		golden.Assert(t, "signin.method."+name, m.View().Content+"\n")

		pressAndSettle(t, m, "t")
		golden.Assert(t, "signin.token."+name, m.View().Content+"\n")
	}
}

// The wizard at the narrow and the medium breakpoints. Below the wordmark's
// width the question leads instead, and nothing is lost.
func TestGoldenTheWizardAtEveryBreakpoint(t *testing.T) {
	t.Parallel()

	for _, bp := range breakpoints {
		th, err := theme.New(theme.Options{Env: termcap.TrueColor, Icons: theme.IconsUnicode})
		require.NoError(t, err)

		m := shell.New(shell.Options{
			Theme: th, Scope: action.ScopeNone, Width: bp.width, Height: bp.height,
			Wizard: onboarding.New(onboarding.Options{
				Theme: th, Service: scriptedSignIn{scope: action.ScopeWrite}, Version: "v0.1.0",
			}),
		})
		golden.Assert(t, "signin."+bp.name, m.View().Content+"\n")
	}
}

// rowFor is a selected row of the kind each screen holds, so a write action is
// refused for its scope rather than for having nothing to act on.
func rowFor(screen action.Screen) action.Row {
	switch screen {
	case action.MergeRequests:
		return action.Row{Kind: action.MergeRequestRow}
	case action.Pipelines:
		return action.Row{Kind: action.PipelineRow}
	case action.Trace:
		return action.Row{Kind: action.JobRow}
	case action.ToDos:
		return action.Row{Kind: action.ToDoRow}
	case action.Issues:
		return action.Row{Kind: action.IssueRow}
	case action.Browse:
		return action.Row{Kind: action.TreeNodeRow}
	default:
		return action.Row{}
	}
}
