package docs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
)

func TestMain(m *testing.M) { harness.Main(m) }

const (
	pagesDir      = "../../website/docs/en"
	ownershipFile = "../../website/docs/OWNERSHIP.md"
)

// ownershipRow matches a row of the table: | `/path` | C01 | C02, C03 |
var ownershipRow = regexp.MustCompile(
	"^\\|\\s*`([^`]+)`\\s*\\|\\s*([^|]+?)\\s*\\|\\s*([^|]*?)\\s*\\|\\s*$")

// changeID matches a roadmap change identifier: C01, C14b, C37a.
var changeID = regexp.MustCompile(`^C\d{2}[ab]?$`)

// The reverse ledger, research/14-test-strategy.md §15.2.
//
// Prevents: a page that no change is obliged to make true. The site is the
// specification, so a page nobody owns is a promise nobody has to keep, and
// that is how a finished-tense site turns into fiction.
func TestEveryPageIsOwnedByAChange(t *testing.T) {
	t.Parallel()

	owners := readOwnership(t)
	pages := readPages(t)

	var unowned []string
	for _, page := range pages {
		if _, ok := owners[page]; !ok {
			unowned = append(unowned, page)
		}
	}
	if len(unowned) > 0 {
		t.Errorf("these pages exist and no change owns them:\n  %s\n\n"+
			"Add a row to website/docs/OWNERSHIP.md naming the change that makes each "+
			"page true, in the same commit that added the page.",
			strings.Join(unowned, "\n  "))
	}
}

// Prevents: the ownership table pointing at a page that was deleted or moved,
// which turns the check above into a check of nothing.
func TestEveryOwnedPageExists(t *testing.T) {
	t.Parallel()

	owners := readOwnership(t)
	pages := map[string]bool{}
	for _, p := range readPages(t) {
		pages[p] = true
	}

	var missing []string
	for page := range owners {
		if !pages[page] {
			missing = append(missing, page)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("website/docs/OWNERSHIP.md names pages that do not exist:\n  %s",
			strings.Join(missing, "\n  "))
	}
}

// Prevents: an owner written as prose rather than as a change, which would make
// the table unreadable by the check above without failing it.
func TestEveryOwnerIsAChangeID(t *testing.T) {
	t.Parallel()

	for page, owner := range readOwnership(t) {
		require.Regexp(t, changeID, owner,
			"%s is owned by %q, which is not a change identifier", page, owner)
	}
}

// This change's own obligation. Prevents: /about/contributing quietly becoming
// somebody else's problem — it is the page that describes the golden-file
// workflow, and nothing later in the roadmap is obliged to write it.
func TestC01OwnsContributing(t *testing.T) {
	t.Parallel()

	owners := readOwnership(t)
	require.Equal(t, "C01", owners["/about/contributing"])
}

// Prevents: a generated page being hand-edited. The two pages listed under
// "Generated pages" come from the Go source, and a change made in the .mdx
// would be silently overwritten by the next regeneration.
func TestGeneratedPagesSayTheyAreGenerated(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(ownershipFile)
	require.NoError(t, err)
	require.Contains(t, string(body), "must never be hand-edited")
	require.Contains(t, string(body), "`labdash keys --markdown`")
}

// readOwnership parses the table into page -> owning change.
func readOwnership(t *testing.T) map[string]string {
	t.Helper()

	body, err := os.ReadFile(ownershipFile)
	require.NoError(t, err, "the ownership table is missing")

	owners := map[string]string{}
	inTable := false

	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "## Generated pages") {
			break
		}
		if strings.HasPrefix(line, "| Page ") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		m := ownershipRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		page, owner := m[1], m[2]
		require.NotContains(t, owners, page, "%s appears twice in the table", page)
		owners[page] = owner
	}

	require.NotEmpty(t, owners, "no rows were read from the ownership table")
	return owners
}

// readPages lists every page on the site, by the route it is served at.
func readPages(t *testing.T) []string {
	t.Helper()

	var pages []string
	err := filepath.WalkDir(pagesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".mdx" {
			return nil
		}

		rel, err := filepath.Rel(pagesDir, path)
		if err != nil {
			return err
		}
		route := "/" + filepath.ToSlash(strings.TrimSuffix(rel, ".mdx"))
		route = strings.TrimSuffix(route, "index")
		pages = append(pages, route)
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, pages, "no pages were found under %s", pagesDir)

	sort.Strings(pages)
	return pages
}
