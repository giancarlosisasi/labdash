// Package updateflag registers -update on the default flag set.
//
// It exists on its own so that every package with a TestMain registers the
// flag, not only the packages that own golden files. Without it,
// `go test ./... -update` fails with "flag provided but not defined" in the
// first package that has no golden of its own — which makes the documented
// workflow in /about/contributing wrong for the whole repository.
package updateflag

import "flag"

var enabled = flag.Bool("update", false,
	"rewrite golden files from the current output instead of comparing against them")

// Enabled reports whether this run rewrites golden files. CI never passes the
// flag: a golden file that no longer matches the screen must fail the build
// rather than rewrite itself.
func Enabled() bool { return *enabled }
