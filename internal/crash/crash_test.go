package crash_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/clock"
	"github.com/giancarlosisasi/labdash/internal/crash"
	"github.com/giancarlosisasi/labdash/internal/testsupport/credguard"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
)

func TestMain(m *testing.M) {
	// The subprocess arm of DIA-04.T1 re-enters this binary and panics on
	// purpose. It must run before the guards, and it never returns.
	if os.Getenv("LABDASH_TEST_PANIC") != "" {
		panicInSubprocess()
	}
	harness.Main(m)
}

// panicInSubprocess is the real crash: a handler installed the way main
// installs it, and a panic underneath it.
func panicInSubprocess() {
	h := crash.New(crash.Options{
		Dir:     os.Getenv("LABDASH_TEST_CRASH_DIR"),
		Version: "0.1.0-test",
		// A restorer that a TUI would register, so the ordering assertion has
		// something of its own to see.
		Out:      os.Stderr,
		Terminal: os.Stdout,
	})
	h.OnRestore(func() { os.Stdout.WriteString("[restorer ran]") })

	defer h.Recover()

	// A credential in scope at the moment of the panic, so that the report is
	// proven not to sweep the environment into itself.
	panic("the ingest section could not be drawn")
}

// DIA-04.T1. Prevents: a broken shell after a crash. This is the whole
// contract: the terminal is restored, a report is written, its path is printed,
// and the exit code is non-zero — in that order, so a failure to write the
// report still leaves a usable shell.
//
// It runs the real binary as a subprocess because an in-process recover proves
// nothing about the exit code, and the exit code is half of what a script that
// invoked labdash needs.
func TestDIA04_T1_APanicLeavesTheTerminalUsable(t *testing.T) {
	dir := t.TempDir()

	cmd := exec.Command(os.Args[0], "-test.run=TestDIA04_T1_APanicLeavesTheTerminalUsable")
	cmd.Env = append(os.Environ(),
		"LABDASH_TEST_PANIC=1",
		"LABDASH_TEST_CRASH_DIR="+dir,
		// A token in the subprocess environment. The report must not carry it.
		"GITLAB_TOKEN=glpat-"+"F7kQ2xLm9pRzT4vWnB8c",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err := cmd.Run()

	// The exit code is non-zero.
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit, "the process exited zero after a panic")
	require.NotZero(t, exit.ExitCode())

	// The terminal was restored, and by the registered restorer as well as by
	// the sequences.
	out := stdout.String()
	require.Contains(t, out, "[restorer ran]", "the registered restorer did not run")
	for name, seq := range map[string]string{
		"leave the alternate screen": "\x1b[?1049l",
		"show the cursor":            "\x1b[?25h",
		"mouse tracking off":         "\x1b[?1000l",
		"bracketed paste off":        "\x1b[?2004l",
		"reset styling":              "\x1b[0m",
	} {
		require.Contains(t, out, seq, "the crash did not %s", name)
	}
	require.Less(t, strings.Index(out, "[restorer ran]"), strings.Index(out, "\x1b[?1049l"),
		"the registered restorer ran after the raw sequences, so a TUI's own "+
			"cleanup would have been undone")

	// The report was written and its path printed.
	reports, err := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	require.NoError(t, err)
	require.Len(t, reports, 1, "no crash report was written")

	diag := stderr.String()
	require.Contains(t, diag, "labdash crashed")
	require.Contains(t, diag, reports[0], "the report's path was not printed")

	// The report says what a bug report needs.
	body, err := os.ReadFile(reports[0])
	require.NoError(t, err)
	report := string(body)

	require.Contains(t, report, "the ingest section could not be drawn")
	require.Contains(t, report, "0.1.0-test")
	require.Contains(t, report, runtime.GOOS+"/"+runtime.GOARCH)
	require.Regexp(t, regexp.MustCompile(`(?m)^stack$`), report)
	require.Contains(t, report, "panic(")

	// Task 6.5: the report passes the credential guard.
	require.Empty(t, credguard.Scan(reports[0], body),
		"the crash report carries a credential-shaped string")
	require.NotContains(t, report, "GITLAB_TOKEN",
		"the report swept the environment into itself")
}

// Prevents: a report that cannot be written taking the terminal restoration
// down with it. The shell comes first for exactly this case.
func TestTheTerminalIsRestoredEvenWhenTheReportCannotBeWritten(t *testing.T) {
	t.Parallel()

	var terminal, out bytes.Buffer
	code := -1

	h := crash.New(crash.Options{
		// A path that is a file, so creating it as a directory fails.
		Dir:      filepath.Join(unwritableDir(t), "report"),
		Out:      &out,
		Terminal: &terminal,
		Clock:    clock.Fixed(time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC)),
		Exit:     func(c int) { code = c },
	})

	func() {
		defer h.Recover()
		panic("boom")
	}()

	require.Contains(t, terminal.String(), "\x1b[?25h", "the cursor was left hidden")
	require.Contains(t, out.String(), "could not be written")
	require.Equal(t, 1, code)
}

// Prevents: a restorer that panics stopping the ones beneath it, or the report.
// Cleanup runs to the end even when part of it is broken.
func TestARestorerThatPanicsDoesNotStopTheRest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var terminal, out bytes.Buffer
	code := -1

	h := crash.New(crash.Options{
		Dir: dir, Out: &out, Terminal: &terminal,
		Clock: clock.Fixed(time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC)),
		Exit:  func(c int) { code = c },
	})
	h.OnRestore(func() { terminal.WriteString("[outer]") })
	h.OnRestore(func() { panic("a restorer failed too") })

	func() {
		defer h.Recover()
		panic("boom")
	}()

	require.Contains(t, terminal.String(), "[outer]")
	require.Contains(t, terminal.String(), "\x1b[?25h")

	reports, err := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, 1, code)
}

// Prevents: the handler firing when nothing went wrong, which would turn a
// clean exit into a crash report.
func TestNoPanicMeansNoReport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	exited := false

	h := crash.New(crash.Options{
		Dir: dir, Out: &bytes.Buffer{}, Terminal: &bytes.Buffer{},
		Exit: func(int) { exited = true },
	})

	func() { defer h.Recover() }()

	require.False(t, exited)
	reports, err := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	require.NoError(t, err)
	require.Empty(t, reports)
}

// Prevents: the report sweeping os.Environ into itself. The environment is
// where a token lives, and a crash report is pasted into a public issue.
func TestOnlyNamedEnvironmentVariablesAreRecorded(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := map[string]string{
		"TERM":         "xterm-256color",
		"COLORTERM":    "truecolor",
		"GITLAB_TOKEN": "glpat-" + "F7kQ2xLm9pRzT4vWnB8c",
		"MY_PAT":       "glpat-" + "R3hN8jVpQ2wLxM7kZa5T",
	}

	h := crash.New(crash.Options{
		Dir: dir, Out: &bytes.Buffer{}, Terminal: &bytes.Buffer{},
		Clock: clock.Fixed(time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC)),
		Exit:  func(int) {},
		Env:   func(k string) string { return env[k] },
	})

	func() {
		defer h.Recover()
		panic("boom")
	}()

	reports, err := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	require.NoError(t, err)
	require.Len(t, reports, 1)

	body, err := os.ReadFile(reports[0])
	require.NoError(t, err)

	require.Contains(t, string(body), "xterm-256color", "the terminal was not recorded")
	require.NotContains(t, string(body), "GITLAB_TOKEN")
	require.NotContains(t, string(body), "MY_PAT")
	require.Empty(t, credguard.Scan(reports[0], body))
}

// unwritableDir returns a path that exists as a file, so that creating a
// directory there fails on every OS.
func unwritableDir(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	return path
}
