package theme

import (
	"os"
	"runtime"
	"strings"
)

// A Tier is the terminal's colour capability, resolved once at startup and
// stored on the theme.
//
// No call site ever branches on it. Every token is already resolved to the tier
// by the time a component reads it, which keeps the branch out of the hottest
// loop in the app and makes a golden file per tier mean something.
//
// research/17-design-system.md §2.1.
type Tier uint8

const (
	// TierUnset is the zero value. A theme carrying it was built without
	// detection, which is a bug rather than a default.
	TierUnset Tier = iota
	// Monochrome emits no colour at all. Hierarchy comes from glyphs, weight
	// and layout. Reached by NO_COLOR, --no-color, TERM=dumb, and the mono
	// theme.
	Monochrome
	// ANSI is sixteen colours, indices 0–15. The user's own terminal theme
	// supplies the hues, which is why labdash looks native everywhere.
	ANSI
	// ANSI256 is the 256-colour cube, nearest-matched from the palette.
	ANSI256
	// TrueColor is 24-bit: the palette exactly as authored.
	TrueColor
)

// String is the name used in a golden file and in `theme preview`.
func (t Tier) String() string {
	switch t {
	case Monochrome:
		return "mono"
	case ANSI:
		return "16"
	case ANSI256:
		return "256"
	case TrueColor:
		return "truecolor"
	default:
		return "unset"
	}
}

// An Environment is the process environment as the theme reads it. Passing it
// rather than calling os.Getenv is what lets one test render the same screen at
// four tiers and two locales without four terminals.
type Environment interface {
	Lookup(key string) (string, bool)
}

// OSEnvironment reads the real process environment.
func OSEnvironment() Environment { return osEnvironment{} }

type osEnvironment struct{}

func (osEnvironment) Lookup(key string) (string, bool) { return os.LookupEnv(key) }

// MapEnvironment is an environment held in memory.
type MapEnvironment map[string]string

// Lookup answers like os.LookupEnv.
func (m MapEnvironment) Lookup(key string) (string, bool) {
	v, ok := m[key]
	return v, ok
}

// NoColorSet reports whether the NO_COLOR standard applies.
//
// The standard is "present and not an empty string, regardless of its value"
// (https://no-color.org/). labdash reads it here rather than delegating,
// because the common library implementation parses it as a boolean and would
// let NO_COLOR=0 turn colour back on. NO_COLOR is unconditional: no setting,
// theme or flag may override it (THM-06, ACC-03).
func NoColorSet(env Environment) bool {
	v, ok := env.Lookup("NO_COLOR")
	return ok && v != ""
}

// DetectTier resolves the terminal's colour capability from the environment.
// The table is research/17-design-system.md §2.1.
func DetectTier(env Environment) Tier {
	if NoColorSet(env) {
		return Monochrome
	}

	term, hasTerm := env.Lookup("TERM")
	if term == "dumb" {
		return Monochrome
	}

	if ct, _ := env.Lookup("COLORTERM"); ct == "truecolor" || ct == "24bit" {
		return TrueColor
	}

	switch {
	case strings.Contains(term, "direct"):
		return TrueColor
	case strings.Contains(term, "256color"):
		return ANSI256
	case hasTerm && term != "":
		return ANSI
	}

	// No TERM at all. On Windows that is normal — the console is not described
	// by terminfo — so the absence of TERM says nothing about capability there.
	if _, ok := env.Lookup("WT_SESSION"); ok {
		// Windows Terminal, which has done 24-bit colour since it shipped.
		return TrueColor
	}
	if runtime.GOOS == "windows" {
		// Any other modern Windows console. 256 colours have worked since
		// Windows 10, and guessing low costs a nicer palette rather than a
		// broken screen.
		return ANSI256
	}

	// On a POSIX system, no TERM means no terminal description. Assume nothing.
	return Monochrome
}
