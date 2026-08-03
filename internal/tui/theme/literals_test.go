package theme_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// themePackage is the one directory permitted to name a colour, relative to the
// module root.
const themePackage = "internal/tui/theme"

// hexColour matches a colour literal as anybody writes one.
var hexColour = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// colourConstructors are the calls that turn a number into a colour. A hex
// string is the obvious way to hardcode one; these are the ways somebody
// reaches for when a string looks too obvious.
var colourConstructors = map[string]bool{
	"lipgloss.Color":      true,
	"lipgloss.ANSIColor":  true,
	"lipgloss.RGBColor":   true,
	"ansi.BasicColor":     true,
	"ansi.ExtendedColor":  true,
	"ansi.TrueColor":      true,
	"color.RGBA":          true,
	"color.NRGBA":         true,
	"colorful.Color":      true,
}

type finding struct {
	file    string
	line    int
	literal string
}

func (f finding) String() string {
	return fmt.Sprintf("%s:%d names a colour: %s", f.file, f.line, f.literal)
}

// THM-01.T1. Prevents: a hardcoded hex at a call site, which makes theming a
// lie — one component that ignores the theme means every theme is wrong on one
// screen, and it is invisible until somebody switches themes.
func TestTHM01_T1_NoColourLiteralOutsideTheThemePackage(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	found, err := scanForColourLiterals(root, themePackage)
	require.NoError(t, err)

	if len(found) > 0 {
		var lines []string
		for _, f := range found {
			lines = append(lines, "  "+f.String())
		}
		t.Fatalf("colour literals exist outside %s:\n%s\n\n"+
			"Name a semantic role from the theme instead. The set is in %s/token.go.",
			themePackage, strings.Join(lines, "\n"), themePackage)
	}
}

// Prevents: a static check that passes because it finds nothing anywhere. The
// rule only holds if the check fires, so it is pointed at a planted literal
// first.
func TestTHM01_T1_TheCheckFiresOnAPlantedLiteral(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "tui", "table"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "internal", "tui", "table", "row.go"),
		[]byte(`package table

import "charm.land/lipgloss/v2"

func failedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#F07178"))
}
`), 0o644))

	// And one file inside the exempt package, which must not be reported.
	require.NoError(t, os.MkdirAll(filepath.Join(root, themePackage), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, themePackage, "palette.go"),
		[]byte("package theme\n\nconst amber = \"#E8A33D\"\n"), 0o644))

	found, err := scanForColourLiterals(root, themePackage)
	require.NoError(t, err)

	require.Len(t, found, 2, "expected the hex string and the lipgloss.Color call")

	var reported []string
	for _, f := range found {
		require.Contains(t, f.file, "row.go",
			"the exempt package was reported, so the rule would fire on the theme itself")
		reported = append(reported, f.literal)
	}
	require.ElementsMatch(t, []string{"#F07178", "lipgloss.Color(…)"}, reported)
}

// scanForColourLiterals walks every Go file under root and reports each colour
// literal outside the exempt directory.
func scanForColourLiterals(root string, exempt ...string) ([]finding, error) {
	exemptSet := map[string]bool{}
	for _, e := range exempt {
		exemptSet[filepath.ToSlash(e)] = true
	}

	var found []finding
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "doc_build", "testdata", ".git", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if exemptSet[filepath.ToSlash(filepath.Dir(rel))] {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			// A file that does not parse is not this test's problem; the build
			// reports it far more clearly.
			return nil //nolint:nilerr
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.BasicLit:
				if node.Kind != token.STRING {
					return true
				}
				v, err := strconv.Unquote(node.Value)
				if err != nil || !hexColour.MatchString(v) {
					return true
				}
				found = append(found, finding{rel, fset.Position(node.Pos()).Line, v})
			case *ast.CallExpr:
				if name, ok := qualifiedName(node.Fun); ok && colourConstructors[name] {
					found = append(found, finding{rel, fset.Position(node.Pos()).Line, name + "(…)"})
				}
			case *ast.CompositeLit:
				if name, ok := qualifiedName(node.Type); ok && colourConstructors[name] {
					found = append(found, finding{rel, fset.Position(node.Pos()).Line, name + "{…}"})
				}
			}
			return true
		})
		return nil
	})

	return found, err
}

// qualifiedName renders pkg.Name for a selector, and reports false for
// anything else.
func qualifiedName(e ast.Expr) (string, bool) {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	return pkg.Name + "." + sel.Sel.Name, true
}

// moduleRoot walks up from the test's directory to the go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "no go.mod above %s", dir)
		dir = parent
	}
}
