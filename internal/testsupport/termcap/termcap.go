// Package termcap fakes the terminal a render believes it is running in.
//
// It supplies the environment the theme reads — colour depth, width, locale —
// so that one render can be exercised at every colour tier and both icon modes
// without four CI runners and four terminals. This is the harness behind
// THM-05, THM-07, ACC-01 and ACC-04: each is proven once here rather than
// twenty times across the screens that inherit it.
//
// See research/14-test-strategy.md §3.6.
package termcap

import (
	"fmt"
	"testing"
)

// A Terminal is everything labdash reads about the terminal it draws into.
// It is a value, not an interface: there is exactly one shape of answer, and a
// struct makes an unset field obvious at the call site.
type Terminal struct {
	// Env is the process environment as the theme would read it: TERM,
	// COLORTERM, NO_COLOR, LC_ALL, LC_CTYPE, LANG.
	Env map[string]string
	// Width and Height are the window size in cells.
	Width, Height int
	// TTY reports whether stdout is a terminal. When false labdash draws no
	// TUI and emits no colour.
	TTY bool
}

// Lookup answers like os.LookupEnv, over the fake environment.
func (t Terminal) Lookup(key string) (string, bool) {
	v, ok := t.Env[key]
	return v, ok
}

// Getenv answers like os.Getenv, over the fake environment.
func (t Terminal) Getenv(key string) string { return t.Env[key] }

// With returns a copy carrying one more environment variable. The receiver is
// untouched, so a table of cases can branch off one base terminal safely from
// parallel tests.
func (t Terminal) With(key, value string) Terminal {
	env := make(map[string]string, len(t.Env)+1)
	for k, v := range t.Env {
		env[k] = v
	}
	env[key] = value
	return Terminal{Env: env, Width: t.Width, Height: t.Height, TTY: t.TTY}
}

// Without returns a copy with key removed.
func (t Terminal) Without(key string) Terminal {
	env := make(map[string]string, len(t.Env))
	for k, v := range t.Env {
		if k != key {
			env[k] = v
		}
	}
	return Terminal{Env: env, Width: t.Width, Height: t.Height, TTY: t.TTY}
}

// Resize returns a copy at a different size.
func (t Terminal) Resize(w, h int) Terminal {
	env := make(map[string]string, len(t.Env))
	for k, v := range t.Env {
		env[k] = v
	}
	return Terminal{Env: env, Width: w, Height: h, TTY: t.TTY}
}

// The four colour tiers, as terminals that produce them. Sizes default to
// 120×32 — the size research/17-design-system.md draws every screen at.
var (
	// TrueColor is a modern terminal: 24-bit colour.
	TrueColor = Terminal{
		Env:   map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor", "LANG": "en_US.UTF-8"},
		Width: 120, Height: 32, TTY: true,
	}
	// ANSI256 is a 256-colour terminal with no COLORTERM.
	ANSI256 = Terminal{
		Env:   map[string]string{"TERM": "xterm-256color", "LANG": "en_US.UTF-8"},
		Width: 120, Height: 32, TTY: true,
	}
	// ANSI16 is a plain terminal: the user's own sixteen colours win.
	ANSI16 = Terminal{
		Env:   map[string]string{"TERM": "xterm", "LANG": "en_US.UTF-8"},
		Width: 120, Height: 32, TTY: true,
	}
	// Monochrome is NO_COLOR. TERM=dumb reaches the same tier.
	Monochrome = Terminal{
		Env:   map[string]string{"TERM": "xterm-256color", "NO_COLOR": "1", "LANG": "en_US.UTF-8"},
		Width: 120, Height: 32, TTY: true,
	}
	// Dumb is the other route to monochrome, and the one a serial console takes.
	Dumb = Terminal{
		Env:   map[string]string{"TERM": "dumb", "LANG": "en_US.UTF-8"},
		Width: 80, Height: 24, TTY: true,
	}
	// Piped is stdout redirected to a file. No TUI, no colour, no splash.
	Piped = Terminal{
		Env:   map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor", "LANG": "en_US.UTF-8"},
		Width: 80, Height: 24, TTY: false,
	}
	// CJK is a terminal whose locale makes East-Asian-Ambiguous glyphs two
	// cells wide. Icon mode auto must resolve to ascii here.
	CJK = Terminal{
		Env:   map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor", "LANG": "ja_JP.UTF-8"},
		Width: 120, Height: 32, TTY: true,
	}
)

// A Tier is one named colour capability, for a table-driven render.
type Tier struct {
	Name     string
	Terminal Terminal
}

// Tiers is the colour matrix every status-bearing screen is rendered through.
// The names are used in golden file names, so they are short and stable.
func Tiers() []Tier {
	return []Tier{
		{"truecolor", TrueColor},
		{"256", ANSI256},
		{"16", ANSI16},
		{"mono", Monochrome},
	}
}

// Matrix runs fn for every colour tier as a subtest. The golden name for each
// is the caller's own prefix plus the tier name, which is what makes "one
// golden per (screen, colour tier, icon mode)" mechanical rather than a
// convention somebody has to remember.
func Matrix(t *testing.T, fn func(t *testing.T, tier Tier)) {
	t.Helper()
	for _, tier := range Tiers() {
		t.Run(tier.Name, func(t *testing.T) {
			t.Helper()
			fn(t, tier)
		})
	}
}

// GoldenName joins a screen name, a colour tier and an icon mode into the
// single filename their golden lives under.
func GoldenName(screen, tier, icons string) string {
	return fmt.Sprintf("%s.%s.%s", screen, tier, icons)
}
