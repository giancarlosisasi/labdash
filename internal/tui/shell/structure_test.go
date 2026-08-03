package shell_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// modelRoots are the packages that hold application state: the root model, the
// contract between it and the views, and the views themselves.
//
// The budget binds these and not the whole tree, because the failure it guards
// against is a root model that absorbs every view. A vocabulary package or a
// renderer growing long is a different problem with a different fix.
var modelRoots = []string{"shell", "view", "views"}

// lineBudget is what one model file may cost. It is deliberately uncomfortable:
// the alternative, arrived at by nobody deciding anything, is a single file
// holding every view's state, and that file is unreadable long before anybody
// notices it happening.
const lineBudget = 250

// SHL-01.T1. Prevents the one architectural smell gh-dash acknowledges: a
// 54 KB ui.go.
func TestSHL01_T1_NoModelFileOutgrowsItsBudget(t *testing.T) {
	t.Parallel()

	var over []string
	for _, path := range modelFiles(t) {
		body, err := os.ReadFile(path)
		require.NoError(t, err)

		if n := bytes.Count(body, []byte("\n")); n > lineBudget {
			over = append(over, filepath.ToSlash(path)+" is "+itoa(n)+" lines")
		}
	}

	require.Empty(t, over, "these files are past the %d-line budget:\n  %s\n\n"+
		"Split by responsibility rather than raising the number.",
		lineBudget, strings.Join(over, "\n  "))
}

// The per-view split, asserted structurally. Prevents the four views drifting
// back into one file with a state struct per view, which is how the budget above
// gets quietly satisfied while the problem gets worse.
func TestSHL01_T1_EachViewModelLivesInItsOwnPackage(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "views")
	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	var packages []string
	for _, e := range entries {
		require.True(t, e.IsDir(), "%s is a loose file among the view packages", e.Name())
		packages = append(packages, e.Name())

		files, err := filepath.Glob(filepath.Join(root, e.Name(), "*.go"))
		require.NoError(t, err)
		require.NotEmpty(t, files, "%s holds no view", e.Name())
	}

	require.ElementsMatch(t, []string{"issues", "mr", "pipelines", "todos"}, packages)
}

// The import direction, asserted so the split survives the first view that needs
// something from the shell. Views depend on the contract in internal/tui/view;
// nothing depends on the root.
func TestSHL01_NoViewImportsTheShell(t *testing.T) {
	t.Parallel()

	const shellPath = "internal/tui/shell"
	fset := token.NewFileSet()

	for _, path := range modelFiles(t) {
		if strings.Contains(filepath.ToSlash(path), shellPath) {
			continue
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		require.NoError(t, err)

		for _, imp := range file.Imports {
			require.NotContains(t, imp.Path.Value, shellPath,
				"%s imports the root model", path)
		}
	}
}

func modelFiles(t *testing.T) []string {
	t.Helper()

	var out []string
	for _, root := range modelRoots {
		err := filepath.WalkDir(filepath.Join("..", root),
			func(path string, _ os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go") {
					out = append(out, path)
				}
				return nil
			})
		require.NoError(t, err)
	}
	require.NotEmpty(t, out, "the check found no model files, so it asserts nothing")
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
