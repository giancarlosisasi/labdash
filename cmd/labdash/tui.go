package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/crash"
	"github.com/giancarlosisasi/labdash/internal/gitlabauth"
	"github.com/giancarlosisasi/labdash/internal/terminal"
	"github.com/giancarlosisasi/labdash/internal/tui/shell"
	"github.com/giancarlosisasi/labdash/internal/tui/theme"
)

// pipedAdvice is the whole of labdash's output when stdout is not a terminal.
// One line, no escape sequences, and it names where machine-readable output
// lives rather than leaving the reader to search for it.
const pipedAdvice = "labdash draws a dashboard and needs a terminal. " +
	"For output you can pipe or parse, use `labdash export`."

// runTUI is the default command: no arguments, a dashboard.
func runTUI(cmd *cobra.Command, handler *crash.Handler) error {
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

	program := tea.NewProgram(
		shell.New(shell.Options{
			Theme:  th,
			Scope:  grantedScope(),
			Width:  width,
			Height: height,
		}),
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

// grantedScope reports what the stored credential may do, so the footer and the
// help overlay offer only what this token can actually reach. A missing
// credential is not an error here: the dashboard opens and says so in a badge.
//
// Telling read_api from api needs a round trip to the instance, which the
// dashboard does not make before its first paint. Until it does, a credential
// that exists is assumed to be able to write, and every mutating action is
// refused for a nearer reason anyway.
func grantedScope() action.Scope {
	if _, err := gitlabauth.Resolve(gitlabauth.Options{}); err != nil {
		return action.ScopeNone
	}
	return action.ScopeWrite
}
