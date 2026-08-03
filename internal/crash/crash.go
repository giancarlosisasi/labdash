// Package crash is what labdash owes the user's shell when it dies.
//
// A panic anywhere in the process restores the terminal, writes a report,
// prints the path to that report, and exits non-zero — in that order. The order
// is the point: a failure to write the report still leaves a usable shell.
//
// The handler is installed in main before anything else, so a panic during
// startup is survivable too.
package crash

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/adrg/xdg"

	"github.com/giancarlosisasi/labdash/internal/clock"
	"github.com/giancarlosisasi/labdash/internal/terminal"
)

// A Restorer puts the terminal back as it was found. The TUI registers one; a
// command that draws no TUI needs none, and the sequences below are emitted
// either way because they are harmless when nothing set them.
type Restorer func()

// Options configure a Handler. The zero value is correct for the real process.
type Options struct {
	// Dir is where reports are written. Empty means the platform state
	// directory.
	Dir string
	// Out is where the report's path is printed. Nil means stderr — the path
	// is a diagnostic, and stdout may be a pipe somebody is parsing.
	Out io.Writer
	// Terminal is where the restore sequences are written. Nil means stdout,
	// which is where the escape sequences that put the terminal into this state
	// came from.
	Terminal io.Writer
	// Version is the build being reported. Empty reads it from the build info.
	Version string
	// Clock names the report file. Nil means the system clock.
	Clock clock.Clock
	// Exit ends the process. Nil means os.Exit. A test replaces it so that the
	// handler can be exercised in-process.
	Exit func(code int)
	// Env reads the terminal environment recorded in the report. Nil means the
	// real one.
	Env func(key string) string
}

// A Handler restores the terminal and writes a report when the process panics.
type Handler struct {
	opts      Options
	restorers []Restorer
}

// New builds a handler. Install it in main before anything else:
//
//	h := crash.New(crash.Options{})
//	defer h.Recover()
func New(opts Options) *Handler {
	if opts.Dir == "" {
		opts.Dir = DefaultDir()
	}
	if opts.Out == nil {
		opts.Out = os.Stderr
	}
	if opts.Terminal == nil {
		opts.Terminal = os.Stdout
	}
	if opts.Clock == nil {
		opts.Clock = clock.System()
	}
	if opts.Exit == nil {
		opts.Exit = os.Exit
	}
	if opts.Env == nil {
		opts.Env = os.Getenv
	}
	if opts.Version == "" {
		opts.Version = buildVersion()
	}
	return &Handler{opts: opts}
}

// OnRestore registers a function that puts the terminal back. The TUI adds its
// own; they run in reverse order of registration, innermost first.
func (h *Handler) OnRestore(f Restorer) { h.restorers = append(h.restorers, f) }

// Recover is deferred in main and in every goroutine labdash starts. A
// goroutine without it takes the process down with the runtime's own message
// and a terminal nobody put back.
func (h *Handler) Recover() {
	v := recover()
	if v == nil {
		return
	}
	h.report(v, debug.Stack())
}

// report performs the four steps in the order that survives its own failures.
func (h *Handler) report(v any, stack []byte) {
	// 1. The terminal, first and unconditionally.
	h.restoreTerminal()

	// 2. The report.
	path, err := h.write(v, stack)

	// 3. The path, or the reason there is none.
	if err != nil {
		fmt.Fprintf(h.opts.Out, "\nlabdash crashed: %v\n", v)
		fmt.Fprintf(h.opts.Out, "The crash report could not be written — %v\n", err)
	} else {
		fmt.Fprintf(h.opts.Out, "\nlabdash crashed: %v\n", v)
		fmt.Fprintf(h.opts.Out, "A report is at %s\n", path)
		fmt.Fprintf(h.opts.Out,
			"It holds the stack, the version and the terminal environment, and no "+
				"credential. Attaching it to an issue is one paste.\n")
	}

	// 4. Non-zero, so a script that ran labdash knows.
	h.opts.Exit(1)
}

// restoreTerminal runs any restorer the TUI registered, innermost first, then
// writes the one set of sequences internal/terminal owns.
func (h *Handler) restoreTerminal() {
	for i := len(h.restorers) - 1; i >= 0; i-- {
		func(f Restorer) {
			// A restorer that panics must not stop the ones beneath it, nor
			// the report.
			defer func() { _ = recover() }()
			f()
		}(h.restorers[i])
	}

	terminal.Restore(h.opts.Terminal)
}

// write produces the report and returns its path.
func (h *Handler) write(v any, stack []byte) (string, error) {
	if err := os.MkdirAll(h.opts.Dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", h.opts.Dir, err)
	}

	at := h.opts.Clock.Now().UTC()
	path := filepath.Join(h.opts.Dir, "crash-"+at.Format("20060102-150405")+".log")

	var b strings.Builder
	fmt.Fprintf(&b, "labdash crash report\n")
	fmt.Fprintf(&b, "when      : %s\n", at.Format(time.RFC3339))
	fmt.Fprintf(&b, "version   : %s\n", h.opts.Version)
	fmt.Fprintf(&b, "go        : %s\n", runtime.Version())
	fmt.Fprintf(&b, "platform  : %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "panic     : %v\n\n", v)

	fmt.Fprintf(&b, "terminal\n")
	for _, key := range reportedEnv {
		if value := h.opts.Env(key); value != "" {
			fmt.Fprintf(&b, "  %-12s %s\n", key, value)
		}
	}

	fmt.Fprintf(&b, "\nstack\n%s\n", stack)

	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// reportedEnv is the environment the report carries, named one variable at a
// time.
//
// It is a list rather than os.Environ() because the environment is where a
// token lives — GITLAB_TOKEN, and whatever a user named in tokenEnv. A crash
// report is pasted into a public issue, so the only safe rule is that nothing
// reaches it unless somebody put it on this list. REG-06 asserts the result.
var reportedEnv = []string{
	"TERM", "COLORTERM", "TERM_PROGRAM", "TERM_PROGRAM_VERSION",
	"WT_SESSION", "ConEmuANSI", "TMUX", "STY", "SSH_TTY",
	"LANG", "LC_ALL", "LC_CTYPE",
	"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE",
	"SHELL", "COMSPEC",
}

// DefaultDir is where reports are written: the platform state directory, beside
// the log.
func DefaultDir() string {
	if dir := os.Getenv("LABDASH_CONFIG_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(xdg.StateHome, "labdash")
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "(devel)"
	}
	return info.Main.Version
}
