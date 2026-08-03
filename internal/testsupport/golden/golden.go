// Package golden compares a rendered frame against a file on disk, and
// rewrites that file when -update is passed.
//
// The reviewable unit is the diff. A golden file changing in a pull request is
// a screen changing, and it is read like code — that is the entire point of the
// mechanism, and it is why CI never passes -update.
//
// See research/14-test-strategy.md §3.2.
package golden

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giancarlosisasi/labdash/internal/testsupport/updateflag"
)

// Updating reports whether this run rewrites golden files. Tests that build
// expensive fixtures only in order to compare can skip that work when true.
func Updating() bool { return updateflag.Enabled() }

// Dir is where a package keeps its golden files, relative to the package
// directory. One directory per package, one file per (screen, colour tier,
// icon mode).
const Dir = "testdata/golden"

// Path returns the file backing name. name is used verbatim, so a test that
// renders the same screen four ways passes four distinct names.
func Path(name string) string {
	return filepath.Join(Dir, name+".golden")
}

// Assert compares got against the golden file for name.
//
// With -update it writes got and passes. Without it, a mismatch fails with a
// line diff and the command that would accept the change.
//
// got is written and compared byte for byte, ANSI colour included: a colour
// regression is the hardest kind to spot by eye and the easiest to catch by
// diff.
func Assert(t testing.TB, name, got string) {
	t.Helper()

	path := Path(name)

	if Updating() {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("golden: creating %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("golden: writing %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden: %s does not exist.\n"+
				"This screen has never been recorded. Read the output below, and if it is\n"+
				"what the screen should look like, run:\n\n    go test ./... -update\n\n%s",
				path, indent(got))
		}
		t.Fatalf("golden: reading %s: %v", path, err)
	}

	if string(want) == got {
		return
	}

	t.Errorf("golden: %s does not match the rendered output.\n"+
		"Read the diff before accepting it — a golden diff is a screen changing.\n"+
		"To accept:\n\n    go test ./... -update\n\n%s",
		path, diff(string(want), got))
}

// Bytes is Assert for output that is not text, such as a captured log file.
func Bytes(t testing.TB, name string, got []byte) {
	t.Helper()
	Assert(t, name, string(got))
}

// diff renders a unified-ish line diff. It is deliberately small: the goal is
// to make the first differing line obvious in CI output, not to reimplement
// git.
func diff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "--- want (%d lines)\n+++ got  (%d lines)\n", len(wantLines), len(gotLines))

	n := max(len(wantLines), len(gotLines))
	for i := range n {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w == g {
			continue
		}
		if i < len(wantLines) {
			fmt.Fprintf(&b, "%4d - %s\n", i+1, escape(w))
		}
		if i < len(gotLines) {
			fmt.Fprintf(&b, "%4d + %s\n", i+1, escape(g))
		}
	}

	return b.String()
}

// escape makes escape sequences visible, so a diff that is only a colour change
// is readable rather than invisible.
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == 0x1b:
			b.WriteString("\\e")
		case r == '\t':
			b.WriteString("\\t")
		case r < 0x20:
			fmt.Fprintf(&b, "\\x%02x", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func indent(s string) string {
	var b bytes.Buffer
	for line := range strings.SplitSeq(s, "\n") {
		b.WriteString("    ")
		b.WriteString(escape(line))
		b.WriteByte('\n')
	}
	return b.String()
}
