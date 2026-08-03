// Package log is labdash's diagnostic output, and it goes to a file.
//
// Nothing here can be made to write to stdout or stderr. There is no console
// handler to construct, no io.Writer to pass in, and no debug flag that
// redirects. That is deliberate: a stray line into a full-screen application
// corrupts the frame, and gh-dash #593 and #942 are exactly that bug. The way
// to keep DIA-03 true is to make the alternative unreachable rather than to
// police call sites.
//
// A caller that wants to see the log opens the file. `labdash doctor` prints
// its path.
package log

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

// Settings is the log: block of settings.yml. The three keys are the whole
// surface: what to record, how big a file may get, and how many to keep.
type Settings struct {
	// Level is debug, info, warn or error. Empty means warn.
	Level string `yaml:"level,omitempty"`
	// MaxSizeMB is the size at which the file rotates. Zero means 10.
	MaxSizeMB int `yaml:"maxSizeMB,omitempty"`
	// MaxFiles is how many rotated files are kept. Zero means 3.
	MaxFiles int `yaml:"maxFiles,omitempty"`
	// Path overrides where the file lives. Empty means the platform state
	// directory. It exists for a test and for a user whose home is read-only,
	// not as a routine setting.
	Path string `yaml:"path,omitempty"`
}

// Defaults, matching research/13-feature-catalog.md §2.
const (
	DefaultLevel     = "warn"
	DefaultMaxSizeMB = 10
	DefaultMaxFiles  = 3
)

// A Logger writes structured records to a file and closes it on shutdown.
type Logger struct {
	*slog.Logger

	path   string
	writer *rotatingFile
}

// Path is where the records are going. It is what an error message and
// `labdash doctor` print.
func (l *Logger) Path() string { return l.path }

// Close flushes and releases the file.
func (l *Logger) Close() error {
	if l == nil || l.writer == nil {
		return nil
	}
	return l.writer.Close()
}

// Open starts logging to a file, creating the directory if it is missing.
//
// It returns an error only for a path it cannot write. A caller that cannot log
// should still run: losing diagnostics is worse than nothing, and refusing to
// start is worse than that.
func Open(s Settings) (*Logger, error) {
	level, err := parseLevel(s.Level)
	if err != nil {
		return nil, err
	}

	path := s.Path
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating the log directory %s: %w", filepath.Dir(path), err)
	}

	size := s.MaxSizeMB
	if size <= 0 {
		size = DefaultMaxSizeMB
	}
	files := s.MaxFiles
	if files <= 0 {
		files = DefaultMaxFiles
	}

	w, err := newRotatingFile(path, size, files)
	if err != nil {
		return nil, err
	}

	return &Logger{
		Logger: slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})),
		path:   path,
		writer: w,
	}, nil
}

// Files lists the log and its rotated backups, which is what a bug report is
// asked to attach.
func (l *Logger) Files() []string {
	if l == nil || l.writer == nil {
		return nil
	}
	return l.writer.existingFiles()
}

// Discard returns a logger that records nothing. It is what a command that has
// not opened a log uses, so no call site ever has to check for nil.
func Discard() *Logger {
	return &Logger{
		Logger: slog.New(slog.DiscardHandler),
		path:   "",
	}
}

// DefaultPath is where the log lives: the platform state directory, which is
// the right home for a file that is regenerated and never edited.
//
// os.UserConfigDir is never used anywhere in labdash — on Windows it returns
// the roaming %APPDATA%, and a log that follows a user between machines is
// wrong. adrg/xdg is what glab uses and parity is the point.
func DefaultPath() string {
	if dir := os.Getenv("LABDASH_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "labdash.log")
	}
	return filepath.Join(xdg.StateHome, "labdash", "labdash.log")
}

func parseLevel(name string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "":
		return slog.LevelWarn, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf(
			"%q is not a log level. It is one of debug, info, warn, error", name)
	}
}
