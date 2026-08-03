package shell_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/crash"
	"github.com/giancarlosisasi/labdash/internal/terminal"
)

// SHL-19.T1. Prevents leaving a user with an unusable shell.
//
// A panic is injected while a Bubble Tea program is running, and the whole path
// is exercised in process: the terminal goes back first, then the report is
// written, then its path is printed, then the exit code is non-zero.
func TestSHL19_T1_APanicInsideALiveSessionRestoresTheTerminal(t *testing.T) {
	t.Parallel()

	var term, out bytes.Buffer
	var code int

	handler := crash.New(crash.Options{
		Dir:      t.TempDir(),
		Terminal: &term,
		Out:      &out,
		Exit:     func(c int) { code = c },
	})

	runAndPanic(t, handler, &term)

	restored := term.String()
	for _, sequence := range terminal.Sequences() {
		require.Contains(t, restored, sequence,
			"the terminal was not put back: %q is missing", sequence)
	}
	require.Contains(t, restored, "\x1b[?25h", "the cursor was left hidden")

	report := out.String()
	require.Contains(t, report, "labdash crashed")
	require.Contains(t, report, "A report is at ")
	require.Equal(t, 1, code, "the process exited zero after a crash")

	path := strings.TrimSpace(strings.SplitN(
		strings.SplitN(report, "A report is at ", 2)[1], "\n", 2)[0])
	body, err := os.ReadFile(path)
	require.NoError(t, err, "the printed path does not exist")
	require.Contains(t, string(body), "the section blew up")
}

// The terminal is put back even when the report cannot be written, because a
// usable shell matters more than the diagnostics.
func TestSHL19_TheTerminalComesBackWhenTheReportCannotBeWritten(t *testing.T) {
	t.Parallel()

	var term, out bytes.Buffer
	var code int

	unwritable := t.TempDir() + string(os.PathSeparator) + "report"
	require.NoError(t, os.WriteFile(unwritable, nil, 0o600))

	handler := crash.New(crash.Options{
		Dir:      unwritable, // a file where a directory is needed
		Terminal: &term,
		Out:      &out,
		Exit:     func(c int) { code = c },
	})

	runAndPanic(t, handler, &term)

	require.Contains(t, term.String(), "\x1b[?25h")
	require.Contains(t, out.String(), "could not be written")
	require.Equal(t, 1, code)
}

// runAndPanic drives a real program to the point where a section panics, with
// the crash handler wired exactly as the binary wires it.
func runAndPanic(t *testing.T, handler *crash.Handler, term io.Writer) {
	t.Helper()

	input, keys := io.Pipe()
	t.Cleanup(func() { _ = input.Close() })

	program := tea.NewProgram(
		explodes{newShell(t, 120, 32)},
		tea.WithInput(input),
		tea.WithOutput(io.Discard),
		tea.WithoutCatchPanics(),
	)
	handler.OnRestore(func() {
		program.Kill()
		terminal.Restore(term)
	})

	go func() {
		_, _ = keys.Write([]byte("x"))
	}()

	func() {
		defer handler.Recover()
		_, _ = program.Run()
	}()
}

// explodes is a section that fails the way a real one would: in the middle of
// handling a keypress, with the screen already drawn.
type explodes struct{ tea.Model }

func (e explodes) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "x" {
		panic("the section blew up")
	}
	next, cmd := e.Model.Update(msg)
	return explodes{next}, cmd
}
