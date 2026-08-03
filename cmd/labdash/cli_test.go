package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/crash"
	"github.com/giancarlosisasi/labdash/internal/keymap"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
)

func TestMain(m *testing.M) { harness.Main(m) }

// anyEscape matches any escape sequence at all, not only colour. A log file
// with a cursor move in it is as broken as one with a colour in it.
var anyEscape = regexp.MustCompile(`\x1b`)

// SHL-23.T1. Prevents escape sequences in a CI log, and a job that blocks
// forever on a prompt nobody can answer.
func TestSHL23_T1_PipedOutputIsOneLineOfPlainText(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	cmd := newRootCmd(crash.New(crash.Options{}), deps{})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	require.NoError(t, cmd.Execute(), "a piped run must exit zero")

	text := out.String()
	require.NotRegexp(t, anyEscape, text, "an escape sequence reached a pipe")
	require.Len(t, strings.Split(strings.TrimSpace(text), "\n"), 1,
		"a piped run printed more than one line")
	require.Contains(t, text, "labdash export",
		"the advice does not name where machine-readable output lives")
}

// The other half of SHL-23: a terminal is never prompted at, so a job that
// somehow reaches this path exits non-zero naming what it needed rather than
// blocking on a question nobody can answer.
func TestSHL23_T1_ATerminalIsNeverPromptedAndTheFailureNamesTheInput(t *testing.T) {
	t.Parallel()

	_, err := readToken(strings.NewReader(""), "gitlab.example.com", true)

	require.Error(t, err, "the command prompted anyway")
	require.Contains(t, err.Error(), "personal access token",
		"the failure does not name the input it needed")
	require.Contains(t, err.Error(), "gitlab.example.com")
}

// A token that arrives on a pipe is read straight through. This is the whole of
// the scripted path, and it is unchanged.
func TestSHL23_APipedTokenIsReadWithoutAPrompt(t *testing.T) {
	t.Parallel()

	token, err := readToken(strings.NewReader("glpat-example\n"), "gitlab.com", false)

	require.NoError(t, err)
	require.Equal(t, "glpat-example", token)
}

// KEY-10.T1 at the command. Prevents the two flags drifting from the keymap
// they both read.
func TestKEY10_T1_TheKeysCommandEmitsTheKeymap(t *testing.T) {
	t.Parallel()

	require.Equal(t, keymap.List(), runKeys(t))
	require.Equal(t, keymap.List(), runKeys(t, "--list"))
	require.Equal(t, keymap.Markdown(), runKeys(t, "--markdown"))

	list := runKeys(t, "--list")
	for _, e := range keymap.Table() {
		require.Contains(t, list, string(e.ID)+"\n")
	}
}

func runKeys(t *testing.T, args ...string) string {
	t.Helper()

	var out bytes.Buffer
	cmd := newKeysCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Not args: a nil slice makes cobra read os.Args, which under `go test`
	// carries the test binary's own flags.
	cmd.SetArgs(append([]string{}, args...))

	require.NoError(t, cmd.Execute())
	return out.String()
}
