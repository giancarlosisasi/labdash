// Package benchharness measures a latency the product promises, and fails the
// build when the promise stops being true.
//
// It exists for PRF-17: keypress to repaint under 50 ms at p99, with 500 rows
// loaded and a fetch in flight. There is no dashboard behind it yet — C08 wires
// one in. What is built here is the measurement, because a benchmark that only
// reports a number is a number nobody reads, and a threshold added after the
// regression is a threshold set to whatever the regression produced.
//
// A p99 rather than a mean: the frame that stutters is the one the user feels,
// and a mean hides one bad frame in ninety-nine good ones.
package benchharness

import (
	"fmt"
	"sort"
	"testing"
	"time"
)

// A Latency collects durations and answers percentile questions about them.
// The zero value is ready to use. It is not safe for concurrent use; a
// benchmark's steps are sequential by construction.
type Latency struct {
	samples []time.Duration
}

// Record adds one measurement.
func (l *Latency) Record(d time.Duration) { l.samples = append(l.samples, d) }

// Time runs step, records how long it took, and returns that duration.
func (l *Latency) Time(step func()) time.Duration {
	start := time.Now()
	step()
	d := time.Since(start)
	l.Record(d)
	return d
}

// Len reports how many samples were taken.
func (l *Latency) Len() int { return len(l.samples) }

// Percentile returns the sample at p, where p is between 0 and 1. It uses the
// nearest-rank method, so P99 of one hundred samples is the worst but one —
// there is no interpolation to argue about.
func (l *Latency) Percentile(p float64) time.Duration {
	if len(l.samples) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), l.samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	rank := int(p*float64(len(sorted))+0.999999) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

// P50, P99 and Max are the three numbers a latency budget is argued about with.
func (l *Latency) P50() time.Duration { return l.Percentile(0.50) }
func (l *Latency) P99() time.Duration { return l.Percentile(0.99) }
func (l *Latency) Max() time.Duration { return l.Percentile(1.0) }

// Summary is the one line a benchmark reports.
func (l *Latency) Summary() string {
	return fmt.Sprintf("n=%d p50=%s p99=%s max=%s",
		l.Len(), round(l.P50()), round(l.P99()), round(l.Max()))
}

func round(d time.Duration) time.Duration {
	if d < time.Millisecond {
		return d.Round(time.Microsecond)
	}
	return d.Round(100 * time.Microsecond)
}

// A Budget is a latency the product promises, with the name it is promised
// under so a failure says which promise broke.
type Budget struct {
	// Name is the feature id and the sentence, e.g.
	// "PRF-17: keypress to repaint".
	Name string
	// P99 is the ceiling. A run whose 99th percentile exceeds it fails.
	P99 time.Duration
	// MinSamples guards against a budget passing on three lucky iterations.
	MinSamples int
}

// Assert fails b when the measured latency misses the budget, and always
// reports the measurement so a passing run still shows the headroom.
//
// It reports through b.ReportMetric as well, so `benchstat` sees the same
// numbers and a slow drift is visible across commits rather than only at the
// moment it crosses the line.
func (bu Budget) Assert(b *testing.B, l *Latency) {
	b.Helper()

	b.ReportMetric(float64(l.P50().Nanoseconds())/1e6, "p50-ms")
	b.ReportMetric(float64(l.P99().Nanoseconds())/1e6, "p99-ms")

	wantSamples := bu.MinSamples
	if wantSamples == 0 {
		wantSamples = 100
	}
	if l.Len() < wantSamples {
		b.Fatalf("%s: %d samples is not enough to trust a p99; wanted at least %d",
			bu.Name, l.Len(), wantSamples)
	}
	if p99 := l.P99(); p99 > bu.P99 {
		b.Fatalf("%s: p99 is %s, over the %s budget (%s)",
			bu.Name, round(p99), bu.P99, l.Summary())
	}
	b.Logf("%s: within budget %s (%s)", bu.Name, bu.P99, l.Summary())
}

// Measure runs step b.N times, timing each call, and asserts the budget.
// It is the whole of a simple latency benchmark:
//
//	func BenchmarkPRF17_Keypress(b *testing.B) {
//	    benchharness.Measure(b, benchharness.Budget{
//	        Name: "PRF-17: keypress to repaint", P99: 50 * time.Millisecond,
//	    }, func() { model.Update(keyDown) })
//	}
func Measure(b *testing.B, budget Budget, step func()) {
	b.Helper()

	var l Latency
	for b.Loop() {
		l.Time(step)
	}

	budget.Assert(b, &l)
}
