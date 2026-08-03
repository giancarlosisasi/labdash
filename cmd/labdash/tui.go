package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/crash"
	"github.com/giancarlosisasi/labdash/internal/gitlabauth"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/terminal"
	"github.com/giancarlosisasi/labdash/internal/tui/onboarding"
	"github.com/giancarlosisasi/labdash/internal/tui/shell"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// version is what the welcome screen shows beside the wordmark. It is a var so
// a release build can stamp it:
//
//	go build -ldflags "-X main.version=v0.1.0"
var version = "dev"

// pipedAdvice is the whole of labdash's output when stdout is not a terminal.
// One line, no escape sequences, and it names where machine-readable output
// lives rather than leaving the reader to search for it.
const pipedAdvice = "labdash draws a dashboard and needs a terminal. " +
	"For output you can pipe or parse, use `labdash export`."

// signInLater is what a user sees when they leave the wizard without finishing.
// It is the whole of the terminal's output in that case: one instruction, and
// no suggestion that anything went wrong.
const signInLater = "Not signed in. Run `labdash auth login` when you are ready, " +
	"or launch labdash again."

// runTUI is the default command: no arguments, a dashboard.
func runTUI(cmd *cobra.Command, handler *crash.Handler, d deps) error {
	// The terminal check happens before anything constructs a Bubble Tea
	// program. Deciding afterwards means the alternate-screen sequence has
	// already gone down the pipe and into somebody's log file.
	out, isFile := cmd.OutOrStdout().(*os.File)
	if !isFile || !term.IsTerminal(int(out.Fd())) {
		fmt.Fprintln(cmd.OutOrStdout(), pipedAdvice)
		return nil
	}

	th, err := theme.New(theme.Options{})
	if err != nil {
		return err
	}

	width, height, err := term.GetSize(int(out.Fd()))
	if err != nil {
		width, height = defaultWidth, defaultHeight
	}

	scope, wizard, check := start(th, d)

	model := shell.New(shell.Options{
		Theme:  th,
		Scope:  scope,
		Wizard: wizard,
		SignIn: signInFor(th, d),
		Verify: check,
		Width:  width,
		Height: height,
	})

	program := tea.NewProgram(
		model,
		tea.WithContext(cmd.Context()),
		// Bubble Tea's own panic catcher restores the terminal and returns an
		// error, and stops there. Letting the panic through means the one crash
		// handler installed in main writes the report and prints its path, which
		// is what turns a crash into something a user can attach to an issue.
		tea.WithoutCatchPanics(),
	)

	// The crash handler owns the restore path. Killing the program first stops
	// the renderer writing over the sequences that put the terminal back.
	handler.OnRestore(func() {
		program.Kill()
		terminal.Restore(out)
	})

	_, err = program.Run()
	terminal.Restore(out)
	return err
}

// start decides what the application opens with: the dashboard, or the wizard.
//
// A missing credential is not an error and never has been an error message
// here. It is a question, and the wizard is where it is asked.
//
// The scope comes from what the instance granted at login, recorded with the
// credential, so read-only is known before the first mutation rather than by
// failing one. A credential whose scopes were never recorded is assumed
// writable; the 403 rewrite is the backstop for that case.
func start(th theme.Theme, d deps) (action.Scope, *onboarding.Model, tea.Cmd) {
	creds, err := gitlabauth.Resolve(gitlabauth.Options{Store: d.Store})
	if err == nil {
		return action.ScopeFromGranted(creds.Scopes), nil, verify(creds)
	}

	return action.ScopeNone, signInFor(th, d)(""), nil
}

// verify asks the instance who the stored credential belongs to, once, after
// the first paint.
//
// It is the only thing standing between a revoked token and a dashboard that
// simply sits there: nothing else in this build makes a request, so nothing
// else would ever notice. A refusal becomes the named error and the recovery
// key; anything else — a flat network, a VPN not up yet — is left alone,
// because signing in again would not fix it.
func verify(creds gitlabauth.Credentials) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		id, err := gitlabauth.WhoAmI(ctx, creds, nil)
		if err == nil {
			return shell.CredentialAccepted{Username: id.Username}
		}

		failure, refused := gitlabauth.ClassifyError(creds, err)
		if !refused {
			return nil
		}

		return shell.CredentialRefused{
			Host:    failure.Host,
			Message: failure.Message(recoveryKey()),
		}
	}
}

// recoveryKey is the key the failure message tells the user to press, read from
// the keymap so the sentence cannot name one that does not exist.
func recoveryKey() string {
	e, ok := keymap.Find(keymap.SignInAgain)
	if !ok {
		return ""
	}
	return keymap.Label(e.Key)
}

// signInFor builds the wizard, for the first run and for the recovery key
// alike. Ctrl+A names the instance that stopped working, so the flow reopens on
// the right host instead of asking again.
func signInFor(th theme.Theme, d deps) func(host string) *onboarding.Model {
	return func(host string) *onboarding.Model {
		cfg, _ := gitlabauth.LoadConfig()

		return onboarding.New(onboarding.Options{
			Theme:   th,
			Service: newSignIn(d),
			Host:    cfg.ResolveHost(host),
			Version: version,
		})
	}
}
