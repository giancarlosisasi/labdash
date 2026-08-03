package log_test

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/log"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
)

func TestMain(m *testing.M) { harness.Main(m) }

// DIA-03.T1. Prevents: frame corruption from a stray log line. gh-dash #593 and
// #942 are that bug, and a full-screen application cannot survive one line of
// output it did not draw.
func TestDIA03_T1_NothingReachesStdoutOrStderr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labdash.log")

	l, err := log.Open(log.Settings{Level: "debug", Path: path})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, l.Close()) })

	stdout, stderr := captureStandardStreams(t, func() {
		l.Debug("refreshing", "section", "platform/*")
		l.Info("fetched", "rows", 120)
		l.Warn("rate limited", "retryAfter", "7s")
		l.Error("could not refresh", "host", "gitlab.example.com")
	})

	require.Empty(t, stdout, "a log record reached stdout")
	require.Empty(t, stderr, "a log record reached stderr")

	recorded, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(recorded), "could not refresh")
	require.Contains(t, string(recorded), "rate limited")
}

// DIA-03.T1, structurally. Prevents: somebody adding a console handler later.
// The rule holds because there is no code path that can build one, not because
// call sites are careful.
func TestNoConsoleHandlerCanBeConstructed(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	require.NoError(t, err)

	forbidden := map[string]bool{
		"os.Stdout": true, "os.Stderr": true,
		"slog.Default": true, "slog.SetDefault": true,
		"fmt.Println": true, "fmt.Printf": true, "fmt.Print": true,
	}

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				if qualified := ident.Name + "." + sel.Sel.Name; forbidden[qualified] {
					t.Errorf("%s:%d names %s. Logging goes to a file and there is no "+
						"path to the terminal — DIA-03",
						filepath.Base(name), fset.Position(sel.Pos()).Line, qualified)
				}
				return true
			})
		}
	}
}

// Prevents: a level that is accepted and then ignored, which is how a debug
// session produces an empty file.
func TestTheLevelFilters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labdash.log")

	l, err := log.Open(log.Settings{Level: "warn", Path: path})
	require.NoError(t, err)

	l.Debug("not recorded")
	l.Info("also not recorded")
	l.Warn("recorded")
	require.NoError(t, l.Close())

	recorded, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(recorded), "not recorded")
	require.Contains(t, string(recorded), "recorded")
}

// Prevents: the default level drifting. warn is the default because an idle
// dashboard should write nothing at all.
func TestTheDefaultLevelIsWarn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labdash.log")

	l, err := log.Open(log.Settings{Path: path})
	require.NoError(t, err)

	l.Info("quiet by default")
	l.Warn("but a warning is kept")
	require.NoError(t, l.Close())

	recorded, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(recorded), "quiet by default")
	require.Contains(t, string(recorded), "but a warning is kept")
}

// Prevents: a log file growing without bound on a machine that runs labdash all
// day. Rotation keeps at most maxFiles files of at most maxSizeMB each.
func TestRotationBoundsWhatIsKept(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "labdash.log")

	l, err := log.Open(log.Settings{Level: "info", MaxSizeMB: 1, MaxFiles: 2, Path: path})
	require.NoError(t, err)

	// Two megabytes of records through a one-megabyte file forces at least one
	// rotation.
	filler := strings.Repeat("x", 4096)
	for range 600 {
		l.Info("filling", "payload", filler)
	}
	require.NoError(t, l.Close())

	entries, err := filepath.Glob(filepath.Join(dir, "labdash*.log"))
	require.NoError(t, err)
	require.Len(t, entries, 2, "expected exactly maxFiles files after rotation")
	require.ElementsMatch(t, entries, l.Files(),
		"the logger disagrees with the directory about which files it kept")

	for _, entry := range entries {
		info, err := os.Stat(entry)
		require.NoError(t, err)
		require.LessOrEqual(t, info.Size(), int64(2*1024*1024),
			"%s grew past its budget", filepath.Base(entry))
	}
}

// Prevents: a level nobody typed on purpose failing with a Go error.
func TestAnUnknownLevelIsExplained(t *testing.T) {
	_, err := log.Open(log.Settings{Level: "verbose", Path: filepath.Join(t.TempDir(), "l.log")})
	require.ErrorContains(t, err, "debug, info, warn, error")
}

// Prevents: a call site having to check for a nil logger. A command that never
// opened a log still logs, into nothing.
func TestDiscardIsUsableAndSilent(t *testing.T) {
	l := log.Discard()

	stdout, stderr := captureStandardStreams(t, func() {
		l.Error("nowhere to go")
	})

	require.Empty(t, stdout)
	require.Empty(t, stderr)
	require.Empty(t, l.Path())
	require.NoError(t, l.Close())
}

// Prevents: the log landing in the roaming profile on Windows, where it would
// follow a user between machines and count against a quota. os.UserConfigDir is
// never used anywhere in labdash for the same reason.
func TestThePathIsUnderTheStateDirectory(t *testing.T) {
	t.Setenv("LABDASH_CONFIG_DIR", filepath.Join(t.TempDir(), "labdash"))
	require.Equal(t,
		filepath.Join(os.Getenv("LABDASH_CONFIG_DIR"), "labdash.log"),
		log.DefaultPath())
}

// Prevents: a record that is not machine-readable. `labdash doctor` and a bug
// report both read this file, and a line that is not JSON is a line neither can
// use.
func TestEveryRecordIsJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labdash.log")

	l, err := log.Open(log.Settings{Level: "info", Path: path})
	require.NoError(t, err)
	l.Info("fetched", "rows", 120, "took", "412ms")
	require.NoError(t, l.Close())

	recorded, err := os.ReadFile(path)
	require.NoError(t, err)

	for _, line := range strings.Split(strings.TrimSpace(string(recorded)), "\n") {
		var doc map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &doc), "line is not JSON: %s", line)
		require.Contains(t, doc, "time")
		require.Contains(t, doc, "level")
		require.Contains(t, doc, "msg")
	}
}

// captureStandardStreams runs fn with both standard streams redirected, and
// returns what each received.
func captureStandardStreams(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	realOut, realErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	errR, errW, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout, os.Stderr = outW, errW

	var outBuf, errBuf bytes.Buffer
	done := make(chan struct{}, 2)
	go func() { _, _ = outBuf.ReadFrom(outR); done <- struct{}{} }()
	go func() { _, _ = errBuf.ReadFrom(errR); done <- struct{}{} }()

	fn()

	os.Stdout, os.Stderr = realOut, realErr
	require.NoError(t, outW.Close())
	require.NoError(t, errW.Close())
	<-done
	<-done
	require.NoError(t, outR.Close())
	require.NoError(t, errR.Close())

	return outBuf.String(), errBuf.String()
}
