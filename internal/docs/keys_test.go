package docs_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/keymap"
)

const keysPage = "../../website/docs/en/keys/index.mdx"

// KEY-10.T1. Prevents a published cheatsheet that is wrong.
//
// /keys/ is generated, so the page and `labdash keys --markdown` are the same
// bytes. Changing a binding without regenerating the page fails here rather
// than shipping a reference that lies.
func TestKEY10_T1_ThePublishedPageIsTheGeneratedKeymap(t *testing.T) {
	t.Parallel()

	onDisk, err := os.ReadFile(keysPage)
	require.NoError(t, err)

	want := keymap.Markdown()
	if normalise(string(onDisk)) == normalise(want) {
		return
	}

	t.Fatalf("%s does not match `labdash keys --markdown`.\n"+
		"The page is generated. Regenerate it rather than editing it:\n\n"+
		"    go run ./cmd/labdash keys --markdown > website/docs/en/keys/index.mdx\n\n"+
		"%s", keysPage, firstDifference(normalise(string(onDisk)), normalise(want)))
}

// The page has to be markdown a reader can pin to a wall, not a Go string with
// tables in it. Prevents an unbalanced table, which MDX renders as a paragraph
// of pipes rather than failing the build.
func TestKEY10_T1_TheGeneratedMarkdownIsWellFormed(t *testing.T) {
	t.Parallel()

	page := keymap.Markdown()

	require.True(t, strings.HasPrefix(page, "---\ntitle: "), "the page has no frontmatter")
	require.Equal(t, 1, strings.Count(page, "\n---\n"), "the frontmatter is not closed exactly once")
	require.Contains(t, page, "GENERATED", "a generated page says so at the top")
	require.True(t, strings.HasSuffix(page, "\n"), "the page does not end in a newline")

	var columns int
	for i, line := range strings.Split(page, "\n") {
		if !strings.HasPrefix(line, "|") {
			columns = 0
			continue
		}
		got := strings.Count(line, "|")
		if columns == 0 {
			columns = got
			continue
		}
		require.Equal(t, columns, got,
			"line %d has %d cell boundaries where its table has %d: %s", i+1, got, columns, line)
	}
}

// KEY-10.T1's second half: the list names every built-in action.
func TestKEY10_T1_TheListNamesEveryAction(t *testing.T) {
	t.Parallel()

	lines := strings.Split(strings.TrimSuffix(keymap.List(), "\n"), "\n")
	require.Len(t, lines, len(keymap.Table()))

	for _, e := range keymap.Table() {
		require.Contains(t, lines, string(e.ID))
	}
	require.IsIncreasing(t, lines, "the list is not sorted, so a diff of it is noise")
}

// normalise removes the line-ending difference a Windows checkout introduces.
// The bytes that matter are the words, not which two of them end a line.
func normalise(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

func firstDifference(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := range max(len(g), len(w)) {
		var a, b string
		if i < len(g) {
			a = g[i]
		}
		if i < len(w) {
			b = w[i]
		}
		if a != b {
			return "first difference, line " + itoa(i+1) + ":\n  page:  " + a + "\n  keymap: " + b
		}
	}
	return ""
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
