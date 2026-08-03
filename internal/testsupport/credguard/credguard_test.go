package credguard_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/credguard"
)

// The test tokens below are synthetic. They are the right shape and no
// instance ever issued them, which is the only safe way to prove a credential
// guard fires.
const (
	fakePAT     = "glpat-" + "F7kQ2xLm9pRzT4vWnB8c"
	fakeDeploy  = "gldt-" + "R3hN8jVpQ2wLxM7kZa5T"
	fakeHex64   = "3f9c1e7a5d2b48660f4a9c3e17d85b20af6c93e14b7d20586caf39e1720d84bc"
	fakeBearer  = "Bearer " + "sk9Lq2Xv7Pm4Rt8Wn3Bz6Yc1"
	fakePrivate = "PRIVATE-TOKEN: " + "Q4mZ8xTv2LpR7Wn3"
)

// REG-06. Prevents: the 2026-08-01 leak, where a test wrote a real token into
// its own output (research/12-current-state.md §9). Each shape here is one the
// guard must catch; a shape that stops matching is a hole in the guard.
func TestREG06_EveryCredentialShapeIsCaught(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, line, pattern string }{
		{"personal access token", "token=" + fakePAT, "personal access token (glpat-)"},
		{"deploy token", "deploy " + fakeDeploy, "deploy token (gldt-)"},
		{"oauth access token", "access_token: " + fakeHex64, "64-character hex token"},
		{"authorization header", "Authorization: " + fakeBearer, "Bearer credential"},
		{"gitlab header", fakePrivate, "PRIVATE-TOKEN header"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			found := credguard.Scan("case", []byte(tc.line))
			require.NotEmpty(t, found, "the guard did not fire on a %s", tc.name)
			require.Equal(t, tc.pattern, found[0].Pattern)
		})
	}
}

// Prevents: a guard that reports the credential it found. A failing build log
// is public, so the report has to name the location without repeating the
// value.
func TestREG06_TheReportNeverRepeatsTheCredential(t *testing.T) {
	t.Parallel()

	found := credguard.Scan("case", []byte("token="+fakePAT))
	require.Len(t, found, 1)

	report := credguard.Report(found)
	require.NotContains(t, report, fakePAT)
	require.Contains(t, report, "credential-shaped characters")
	require.Contains(t, report, "REG-06")
}

// Prevents: a guard so eager that somebody switches it off. A commit hash, a
// project path and the redaction the HTTP logger writes must all pass.
func TestREG06_OrdinaryContentIsNotACredential(t *testing.T) {
	t.Parallel()

	for _, line := range []string{
		"commit 4f2a91c",
		"platform/ingest!2841",
		"Authorization: Bearer [redacted]",
		"PRIVATE-TOKEN: [redacted]",
		"https://gitlab.example.com/platform/ingest/-/merge_requests/2841",
		"passed  failed  running  pending",
	} {
		require.Empty(t, credguard.Scan("case", []byte(line)),
			"the guard fired on ordinary content: %s", line)
	}
}

// Prevents: a guard that reads only the file it was pointed at. Golden files,
// fixtures and captured logs all live under testdata, and the walk has to find
// a credential at any depth.
func TestREG06_ScanDirWalksTestdata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	deep := filepath.Join(root, "golden", "nested")
	require.NoError(t, os.MkdirAll(deep, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(deep, "screen.golden"), []byte("header\ntoken="+fakePAT+"\n"), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "clean.json"), []byte(`{"project":"platform/ingest"}`), 0o644))

	found, err := credguard.ScanDir(root)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, 2, found[0].Line)
	require.Contains(t, found[0].Source, "screen.golden")
}

// Prevents: a missing testdata directory failing every package that has none.
func TestREG06_AMissingDirectoryIsNotAFailure(t *testing.T) {
	t.Parallel()

	found, err := credguard.ScanDir(filepath.Join(t.TempDir(), "does-not-exist"))
	require.NoError(t, err)
	require.Empty(t, found)
}

// Prevents: the stream scanner swallowing or reordering the output it checks.
// It sits in front of stdout for the whole suite, so it has to be a faithful
// pipe first and a scanner second.
func TestREG06_TheStreamScannerPassesEveryByteThrough(t *testing.T) {
	t.Parallel()

	in := "--- PASS: TestOne\nleaked=" + fakePAT + "\n--- PASS: TestTwo\n"

	var out bytes.Buffer
	s := credguard.NewStreamScanner("stdout")
	s.Copy(&out, strings.NewReader(in))

	require.Equal(t, in, out.String(), "the tee changed the output it was copying")

	found := s.Findings()
	require.Len(t, found, 1)
	require.Equal(t, 2, found[0].Line)
	require.Equal(t, "stdout", found[0].Source)
}
