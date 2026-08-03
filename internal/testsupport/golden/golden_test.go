package golden_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/golden"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
)

// These tests change the working directory and the -update flag, so none of
// them is parallel.
func TestMain(m *testing.M) { harness.Main(m) }

// stopped is what a fake Fatalf panics with, standing in for the runtime.Goexit
// the real one performs.
type stopped struct{}

// recorder stands in for *testing.T so a test can assert on a golden failure
// without failing itself. Embedding testing.TB satisfies the interface's
// unexported method; the two reporting methods are overridden.
type recorder struct {
	testing.TB
	msgs []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.msgs = append(r.msgs, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.msgs = append(r.msgs, fmt.Sprintf(format, args...))
	panic(stopped{})
}

func (r *recorder) failed() bool  { return len(r.msgs) > 0 }
func (r *recorder) report() string { return strings.Join(r.msgs, "\n") }

// capture runs fn against a recorder, absorbing the stop a Fatalf performs.
func capture(t *testing.T, fn func(tb testing.TB)) *recorder {
	t.Helper()

	r := &recorder{TB: t}
	func() {
		defer func() {
			if v := recover(); v != nil {
				if _, ok := v.(stopped); !ok {
					panic(v)
				}
			}
		}()
		fn(r)
	}()
	return r
}

// Prevents: a golden helper that passes when the file does not exist, which
// would let a screen ship with no recorded appearance at all.
func TestAMissingGoldenFails(t *testing.T) {
	inDir(t, t.TempDir())

	skipWhenUpdating(t)

	r := capture(t, func(tb testing.TB) {
		golden.Assert(tb, "never-recorded", "some frame")
	})

	require.True(t, r.failed(), "a missing golden file passed")
	require.Contains(t, r.report(), "-update")
	require.Contains(t, r.report(), "never been recorded")
}

// Prevents: a comparison that normalises whitespace or strips ANSI. Colour is
// in the file on purpose — research/14-test-strategy.md §3.2 — because a
// colour regression is the hardest kind to see by eye and the easiest to diff.
func TestComparisonIsByteForByteIncludingColour(t *testing.T) {
	dir := t.TempDir()
	inDir(t, dir)

	skipWhenUpdating(t)

	const amber = "\x1b[38;2;232;163;61mpassed\x1b[0m"
	const green = "\x1b[38;2;123;216;143mpassed\x1b[0m"

	writeGolden(t, "coloured", amber)
	golden.Assert(t, "coloured", amber)

	r := capture(t, func(tb testing.TB) { golden.Assert(tb, "coloured", green) })

	require.True(t, r.failed(), "a colour-only change passed the comparison")
	require.Contains(t, r.report(), `\e[38;2;232;163;61m`,
		"the diff did not make the escape sequence readable")
}

// Prevents: -update being a flag that does not actually rewrite, which is only
// discovered when somebody has already committed a stale golden file.
// skipWhenUpdating steps aside for a test that asserts a golden *failure*.
// With -update on there is nothing to fail: Assert writes rather than compares,
// which is the behaviour the tests below this one prove.
func skipWhenUpdating(t *testing.T) {
	t.Helper()
	if golden.Updating() {
		t.Skip("-update rewrites rather than compares, so there is no failure to assert")
	}
}

func TestUpdateRewritesTheFile(t *testing.T) {
	inDir(t, t.TempDir())

	writeGolden(t, "reflowed", "an old frame")

	withUpdate(t, func() {
		golden.Assert(t, "reflowed", "a new frame")
	})

	got, err := os.ReadFile(golden.Path("reflowed"))
	require.NoError(t, err)
	require.Equal(t, "a new frame", string(got))

	// And the rewritten file is what a later run compares against.
	golden.Assert(t, "reflowed", "a new frame")
}

// Prevents: -update refusing to record a screen that has never been recorded,
// which would make the first golden for every new screen a hand-written file.
func TestUpdateCreatesTheDirectory(t *testing.T) {
	inDir(t, t.TempDir())

	withUpdate(t, func() {
		golden.Assert(t, "brand-new", "first frame")
	})

	require.FileExists(t, golden.Path("brand-new"))
}

// Prevents: a diff that names no line number, which turns a 40-row screen
// regression into a hunt.
func TestTheDiffNamesTheLineThatMoved(t *testing.T) {
	inDir(t, t.TempDir())

	writeGolden(t, "table", "one\ntwo\nthree")

	skipWhenUpdating(t)

	r := capture(t, func(tb testing.TB) { golden.Assert(tb, "table", "one\nTWO\nthree") })

	require.True(t, r.failed())
	require.Contains(t, r.report(), "   2 - two")
	require.Contains(t, r.report(), "   2 + TWO")
	require.NotContains(t, r.report(), "1 - one",
		"an unchanged line appeared in the diff")
}

// Prevents: one golden file being shared by two colour tiers, which is how a
// tier stops being tested without anybody noticing.
func TestOneFilePerName(t *testing.T) {
	inDir(t, t.TempDir())

	require.NotEqual(t, golden.Path("preview.truecolor.unicode"),
		golden.Path("preview.truecolor.ascii"))
	require.Equal(t,
		filepath.Join("testdata", "golden", "preview.16.ascii.golden"),
		golden.Path("preview.16.ascii"))
}

func writeGolden(t *testing.T, name, content string) {
	t.Helper()
	path := golden.Path(name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// withUpdate runs fn as though the suite had been started with -update.
func withUpdate(t *testing.T, fn func()) {
	t.Helper()
	before := golden.Updating()
	require.NoError(t, flag.Set("update", "true"))
	defer func() {
		require.NoError(t, flag.Set("update", fmt.Sprint(before)))
	}()
	fn()
}

// inDir runs the rest of the test with dir as the working directory, so the
// helper's relative testdata/golden path lands somewhere disposable.
func inDir(t *testing.T, dir string) {
	t.Helper()
	before, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(before)) })
}
