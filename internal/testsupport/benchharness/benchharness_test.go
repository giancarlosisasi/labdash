package benchharness_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/benchharness"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
)

func TestMain(m *testing.M) { harness.Main(m) }

// Prevents: a p99 that is really a mean. PRF-17 promises keypress to repaint
// under 50 ms *at p99*, and a mean hides a cluster of slow frames inside a
// crowd of fast ones. One slow frame in a hundred is the 1% a p99 forgives; two
// is not, and the budget has to notice the difference.
func TestP99CatchesSlowFramesAMeanWouldHide(t *testing.T) {
	t.Parallel()

	one := latency(99, time.Millisecond, 1, 500*time.Millisecond)
	require.Equal(t, time.Millisecond, one.P50())
	require.Equal(t, time.Millisecond, one.P99(),
		"one slow frame in a hundred is the 1 percent a p99 forgives")
	require.Equal(t, 500*time.Millisecond, one.Max(),
		"the outlier vanished entirely; max is what makes it visible")

	two := latency(98, time.Millisecond, 2, 500*time.Millisecond)
	require.Equal(t, time.Millisecond, two.P50(),
		"a mean would have moved here; a median must not")
	require.Equal(t, 500*time.Millisecond, two.P99(),
		"two slow frames in a hundred were averaged away")
}

func latency(fast int, fastD time.Duration, slow int, slowD time.Duration) *benchharness.Latency {
	var l benchharness.Latency
	for range fast {
		l.Record(fastD)
	}
	for range slow {
		l.Record(slowD)
	}
	return &l
}

// Prevents: nearest-rank arithmetic that is off by one, which is how a budget
// silently reports the second-worst sample and passes.
func TestPercentileUsesNearestRank(t *testing.T) {
	t.Parallel()

	var l benchharness.Latency
	for i := 1; i <= 100; i++ {
		l.Record(time.Duration(i) * time.Millisecond)
	}

	require.Equal(t, 50*time.Millisecond, l.P50())
	require.Equal(t, 99*time.Millisecond, l.P99())
	require.Equal(t, 100*time.Millisecond, l.Max())
}

// Prevents: an empty run reporting zero and passing a budget it never measured.
func TestAnEmptyRunHasNoPercentile(t *testing.T) {
	t.Parallel()

	var l benchharness.Latency
	require.Zero(t, l.Len())
	require.Zero(t, l.P99())
}

// The harness measuring itself: this is the shape C08 wires a real dashboard
// into for PRF-17.
func BenchmarkHarnessMeasuresAndReports(b *testing.B) {
	benchharness.Measure(b, benchharness.Budget{
		Name:       "harness self-check",
		P99:        50 * time.Millisecond,
		MinSamples: 1,
	}, func() {
		_ = make([]byte, 64)
	})
}
