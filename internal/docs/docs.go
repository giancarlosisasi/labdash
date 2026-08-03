// Package docs holds the checks that keep the documentation site honest.
//
// The site is written in the finished tense and describes the product as it
// will be, which makes every page a commitment. The check that matters is that
// no page is a promise nobody owns: `website/docs/OWNERSHIP.md` names the
// change that must make each page true, and the test in this package asserts
// that the table and the files on disk are the same set.
//
// There is no production code here. The test is the whole package, because the
// thing being asserted is a property of the repository rather than of a
// running binary.
//
// research/14-test-strategy.md §15.2.
package docs
