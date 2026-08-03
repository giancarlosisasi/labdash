package clock_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/clock"
)

// Prevents: a fixed clock that drifts, which would make every golden file
// containing a timestamp flake instead of fail.
func TestFixedClockDoesNotMove(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC)
	c := clock.Fixed(at)

	require.True(t, c.Now().Equal(at))
	time.Sleep(2 * time.Millisecond)
	require.True(t, c.Now().Equal(at), "a fixed clock moved between two reads")
}

// Prevents: a package-level `var now = time.Now` that parallel tests cannot
// each pin to their own instant. research/14-test-strategy.md §3.3 requires a
// clock per test case because THM-10.T1 runs several timezones at once.
func TestFixedClockIsSafeFromParallelTests(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC)
	c := clock.Fixed(at)

	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !c.Now().Equal(at) {
				t.Errorf("a concurrent read saw %s, not %s", c.Now(), at)
			}
		}()
	}
	wg.Wait()
}

// Prevents: a test walking time forward by mutating a shared clock, which
// forces synchronisation on every reader.
func TestOffsetWalksTimeWithoutMutation(t *testing.T) {
	t.Parallel()

	base := clock.Fixed(time.Date(2026, 8, 2, 14, 30, 0, 0, time.UTC))
	later := clock.Offset(base, 90*time.Minute)

	require.Equal(t, "2026-08-02T14:30:00Z", base.Now().Format(time.RFC3339))
	require.Equal(t, "2026-08-02T16:00:00Z", later.Now().Format(time.RFC3339))
}

// Prevents: System() quietly returning a fixed clock, which would freeze the
// running application while every test still passed.
func TestSystemClockAdvances(t *testing.T) {
	t.Parallel()

	c := clock.System()
	first := c.Now()
	time.Sleep(2 * time.Millisecond)
	require.True(t, c.Now().After(first), "the system clock did not advance")
}
