package credguard_test

import (
	"testing"

	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
)

// This package's own tests hold credential-shaped strings on purpose, so the
// output scan stays on: it proves the guard never prints what it found. What
// is switched off is the testdata walk, because this package writes its
// fixtures into t.TempDir rather than into testdata.
func TestMain(m *testing.M) { harness.Main(m, harness.ScanDirs()) }
