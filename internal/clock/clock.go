// Package clock is the only place labdash reads the current time.
//
// Nothing calls time.Now directly. A Clock is threaded through the program
// context and tests inject a fixed one, which is what makes a rendered frame
// byte-identical across runs and therefore worth keeping in a golden file.
// Relative timestamps, token-expiry warnings and refresh intervals are all
// pure functions of the injected clock.
//
// See research/14-test-strategy.md §3.3. Retrofitting this later would touch
// every file that renders a date, which is why it exists before the first
// screen does.
package clock

import "time"

// Clock reads wall time. Production wires System; tests wire Fixed.
type Clock interface {
	// Now returns the current time, in the location the clock carries.
	Now() time.Time
}

// System is the real clock. It is the only caller of time.Now in labdash.
func System() Clock { return systemClock{} }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Fixed returns a clock that always reports at. It is immutable, so any number
// of parallel tests may share one value without synchronising.
//
// Fixed lives beside the interface rather than in internal/testsupport because
// production code must never import test-only packages, and because a clock
// pinned to a constant is useful outside a test — a --now flag, a replayed
// session, a deterministic export.
func Fixed(at time.Time) Clock { return fixedClock{at: at} }

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

// Offset returns a clock reporting at a constant delta from base. It is how a
// test walks time forward without a mutable clock and the synchronisation a
// mutable clock would need.
func Offset(base Clock, d time.Duration) Clock { return offsetClock{base: base, d: d} }

type offsetClock struct {
	base Clock
	d    time.Duration
}

func (c offsetClock) Now() time.Time { return c.base.Now().Add(c.d) }
