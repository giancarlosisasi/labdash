// Package harness is the TestMain every labdash package runs.
//
// It installs the two suite-wide guards at once:
//
//   - goleak, so a goroutine that outlives its test fails the package that
//     leaked it rather than the package that happens to run last. A dashboard
//     firing six concurrent fetches on a timer is exactly the shape that leaks,
//     and a leak is invisible until a session has been open for eight hours.
//   - credguard (REG-06), over both the package's testdata and everything the
//     test binary wrote to stdout and stderr.
//
// Usage, in one file per package:
//
//	func TestMain(m *testing.M) { harness.Main(m) }
//
// See research/14-test-strategy.md §3.4 and §3.5.
package harness

import (
	"fmt"
	"os"
	"testing"

	"go.uber.org/goleak"

	"github.com/giancarlosisasi/labdash/internal/testsupport/credguard"
	// Importing updateflag registers -update on the default flag set for every
	// package that has a TestMain, so `go test ./... -update` reaches packages
	// with no golden files of their own instead of failing on an unknown flag
	// in the first one it meets.
	_ "github.com/giancarlosisasi/labdash/internal/testsupport/updateflag"
)

// An Option adjusts what Main guards.
type Option func(*config)

type config struct {
	scanOutput bool
	scanDirs   []string
	leakOpts   []goleak.Option
}

// WithoutOutputScan stops Main from teeing stdout and stderr through the
// credential scanner. Reach for it only when a test deliberately writes a
// credential-shaped string as part of proving the guard works.
func WithoutOutputScan() Option {
	return func(c *config) { c.scanOutput = false }
}

// ScanDirs replaces the set of directories credguard walks. The default is
// testdata, which is where golden files, fixtures and captured logs live.
func ScanDirs(dirs ...string) Option {
	return func(c *config) { c.scanDirs = dirs }
}

// IgnoreGoroutine excuses one goroutine from the leak check by the function it
// is parked in. Every call needs a comment naming why that goroutine outlives
// the suite; an unexplained entry is how goleak stops finding anything.
func IgnoreGoroutine(fn string) Option {
	return func(c *config) { c.leakOpts = append(c.leakOpts, goleak.IgnoreTopFunction(fn)) }
}

// Main runs the package's tests under both guards and exits.
func Main(m *testing.M, opts ...Option) {
	cfg := config{scanOutput: true, scanDirs: []string{"testdata"}}
	for _, o := range opts {
		o(&cfg)
	}

	var (
		outScan, errScan *credguard.StreamScanner
		restore          func()
	)
	if cfg.scanOutput {
		outScan, errScan, restore = teeStandardStreams()
	}

	code := m.Run()

	if restore != nil {
		restore()
	}

	var findings []credguard.Finding
	if outScan != nil {
		findings = append(findings, outScan.Findings()...)
		findings = append(findings, errScan.Findings()...)
	}
	for _, dir := range cfg.scanDirs {
		found, err := credguard.ScanDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "REG-06: scanning %s: %v\n", dir, err)
			code = 1
		}
		findings = append(findings, found...)
	}
	if len(findings) > 0 {
		fmt.Fprint(os.Stderr, credguard.Report(findings))
		code = 1
	}

	// goleak runs last, after the tee goroutines have been joined, so it never
	// reports the guard's own plumbing.
	if code == 0 {
		if err := goleak.Find(cfg.leakOpts...); err != nil {
			fmt.Fprintf(os.Stderr, "goleak: a goroutine outlived the test suite:\n%v\n", err)
			code = 1
		}
	}

	os.Exit(code)
}

// teeStandardStreams replaces os.Stdout and os.Stderr with pipes that copy
// through to the real files while scanning. restore puts them back and waits
// for both copiers to finish, so nothing is lost and no goroutine is left for
// goleak to find.
func teeStandardStreams() (out, errS *credguard.StreamScanner, restore func()) {
	realOut, realErr := os.Stdout, os.Stderr

	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, nil, nil
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		outR.Close()
		outW.Close()
		return nil, nil, nil
	}

	out = credguard.NewStreamScanner("stdout")
	errS = credguard.NewStreamScanner("stderr")

	os.Stdout, os.Stderr = outW, errW

	done := make(chan struct{}, 2)
	go func() { out.Copy(realOut, outR); done <- struct{}{} }()
	go func() { errS.Copy(realErr, errR); done <- struct{}{} }()

	return out, errS, func() {
		os.Stdout, os.Stderr = realOut, realErr
		outW.Close()
		errW.Close()
		<-done
		<-done
		outR.Close()
		errR.Close()
	}
}
