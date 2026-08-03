package terminal_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/terminal"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
)

func TestMain(m *testing.M) { harness.Main(m) }

// The state a TUI leaves behind, and what has to be undone. Prevents a
// sequence quietly disappearing from the list, which is invisible until
// somebody's shell stops echoing.
func TestRestoreUndoesEverythingATUISets(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	terminal.Restore(&out)

	for name, sequence := range map[string]string{
		"leaving the alternate screen": "\x1b[?1049l",
		"showing the cursor":           "\x1b[?25h",
		"mouse tracking off":           "\x1b[?1000l",
		"button-event tracking off":    "\x1b[?1002l",
		"any-event tracking off":       "\x1b[?1003l",
		"SGR mouse mode off":           "\x1b[?1006l",
		"bracketed paste off":          "\x1b[?2004l",
		"line wrap on":                 "\x1b[?7h",
		"styling reset":                "\x1b[0m",
	} {
		require.Contains(t, out.String(), sequence, "%s is missing", name)
	}
}

// One restore path, not three. Prevents the copy in the crash handler and the
// copy in the quit path drifting until only one of them turns mouse reporting
// off.
func TestOnlyOnePlaceKnowsTheRestoreSequences(t *testing.T) {
	t.Parallel()

	var offenders []string
	root := filepath.Join("..", "..")

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "doc_build" || d.Name() == ".git") {
			return filepath.SkipDir
		}
		if filepath.Ext(path) != ".go" || strings.Contains(path, filepath.Join("internal", "terminal")) {
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// The alternate screen is the sequence a second implementation always
		// starts with, so it is the one worth watching for.
		if bytes.Contains(body, []byte(`\x1b[?1049l`)) && !strings.HasSuffix(path, "_test.go") {
			offenders = append(offenders, path)
		}
		return nil
	})
	require.NoError(t, err)

	require.Empty(t, offenders,
		"these files write the restore sequences themselves; call terminal.Restore instead")
}
