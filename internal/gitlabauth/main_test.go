package gitlabauth

import (
	"testing"

	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
)

// This package talks to an OAuth server and holds credentials, so it is the
// one that most needs both guards: goleak for the device-flow poller, and
// REG-06 for everything the tests print. The 2026-08-01 leak was here.
func TestMain(m *testing.M) { harness.Main(m) }
