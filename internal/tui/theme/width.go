package theme

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Width returns the number of terminal cells s occupies, counting escape
// sequences as zero and wide characters as two.
//
// This and Truncate are the only place display text is measured or cut. Any
// other slicing of a string that reaches the screen is a bug, and a reviewer
// has exactly one pair of functions to look for. A table that measures a
// project name with len() drifts one column for every CJK character in it.
func Width(s string) int { return ansi.StringWidth(s) }

// Truncate cuts s to at most w cells, ending in tail when anything was
// removed.
//
// It never hard-clips and never cuts inside a wide character, a combining
// sequence or an escape sequence. Pass the tier's Ellipsis as tail.
func Truncate(s string, w int, tail string) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, tail)
}

// Wrap breaks s into lines of at most w cells, on word boundaries.
//
// It is for the one kind of text that must never be truncated: an error or an
// empty state, where the part that gets cut is the part telling the reader what
// to do about it. Everything in a column is truncated instead — a wrapped table
// cell would change a row's height.
func Wrap(s string, w int) []string {
	if w <= 0 || s == "" {
		return nil
	}
	return strings.Split(ansi.Wordwrap(s, w, " -"), "\n")
}

// Pad returns s padded with spaces to exactly w cells, truncating with tail
// when it is too long.
//
// Exactly w: a cell that renders one column wider than it was allotted moves
// every column after it, which is the bug that makes tables drift for anyone
// with a non-Latin project name (ACC-09).
func Pad(s string, w int, tail string) string {
	if w <= 0 {
		return ""
	}
	s = Truncate(s, w, tail)
	if gap := w - Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// PadLeft is Pad for a column aligned to the right: numbers, durations, counts.
// A right-aligned duration column is the difference between a table you can
// scan and a table you have to read.
func PadLeft(s string, w int, tail string) string {
	if w <= 0 {
		return ""
	}
	s = Truncate(s, w, tail)
	if gap := w - Width(s); gap > 0 {
		return strings.Repeat(" ", gap) + s
	}
	return s
}

// Center pads s to w cells with the content centred, which is how a
// single-glyph status column is aligned.
func Center(s string, w int, tail string) string {
	if w <= 0 {
		return ""
	}
	s = Truncate(s, w, tail)
	gap := w - Width(s)
	if gap <= 0 {
		return s
	}
	left := gap / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", gap-left)
}

// asciiFolds are the typographic characters labdash's copy uses, and their
// ASCII spellings.
//
// It is a small closed set on purpose: prose acquires an ellipsis or an em dash
// the moment somebody writes a sentence, and ACC-04's promise that no byte
// above 0x7F reaches an ASCII-mode screen has to hold for words as well as for
// glyphs. Folding in one place is what keeps that true without every string
// literal having to think about it.
var asciiFolds = strings.NewReplacer(
	"…", "...",
	"—", "--",
	"–", "-",
	"·", "|",
	"’", "'",
	"“", `"`,
	"”", `"`,
	"×", "x",
	"→", "->",
)

// FoldASCII rewrites user-facing text for the ASCII tier. Text already inside
// the ASCII range passes through untouched.
func FoldASCII(s string, tier IconTier) string {
	if tier != ASCII {
		return s
	}
	return asciiFolds.Replace(s)
}

// The three spacing values, and there are only three. More than three is not a
// system, it is a habit. research/17-design-system.md §5.
const (
	// SpaceTight sits between a glyph and its label.
	SpaceTight = 1
	// SpaceNormal sits between table columns.
	SpaceNormal = 2
	// SpaceLoose sits between logical groups in the footer and context bar.
	SpaceLoose = 4
)
