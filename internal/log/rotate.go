package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// A rotatingFile is an io.WriteCloser that keeps a log within bounds.
//
// It rotates in the caller's goroutine and starts none of its own. That is a
// deliberate choice over the obvious dependency: gopkg.in/natefinch/lumberjack
// spawns a background mill per logger and offers no way to stop it, so goleak
// reports one leaked goroutine per logger and the only options are an
// unexplained exemption or this. Rotation by size is a rename and a prune, and
// the risk of writing it is smaller than the risk of a rule we stop enforcing.
//
// Rotation closes the file before renaming it, because Windows refuses to
// rename a file that is still open. That is the one platform detail in here and
// it is the reason the sequence is close, rename, reopen rather than the other
// way round.
type rotatingFile struct {
	path    string
	maxSize int64 // bytes
	keep    int   // total files retained, current included

	mu   sync.Mutex
	file *os.File
	size int64
}

func newRotatingFile(path string, maxSizeMB, keep int) (*rotatingFile, error) {
	r := &rotatingFile{
		path:    path,
		maxSize: int64(maxSizeMB) * 1024 * 1024,
		keep:    keep,
	}
	if err := r.open(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil {
		return 0, os.ErrClosed
	}

	// A single record longer than the whole budget is written anyway: losing it
	// would hide the one event big enough to matter.
	if r.size > 0 && r.size+int64(len(p)) > r.maxSize {
		if err := r.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := r.file.Write(p)
	r.size += int64(n)
	return n, err
}

func (r *rotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}

func (r *rotatingFile) open() error {
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("opening the log file %s: %w", r.path, err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("reading the log file %s: %w", r.path, err)
	}

	r.file, r.size = f, info.Size()
	return nil
}

// rotate moves the current file aside and starts a new one. The caller holds
// the lock.
func (r *rotatingFile) rotate() error {
	if err := r.file.Close(); err != nil {
		return err
	}
	r.file = nil

	// Shift the backups down: .2 becomes .3, .1 becomes .2, and the current
	// file becomes .1. Counting down means nothing is overwritten on the way.
	for i := r.keep - 1; i >= 1; i-- {
		from := r.backupPath(i)
		to := r.backupPath(i + 1)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		if i+1 >= r.keep {
			// Past the budget: this one is dropped rather than shifted.
			if err := os.Remove(from); err != nil {
				return err
			}
			continue
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}

	if r.keep > 1 {
		if err := os.Rename(r.path, r.backupPath(1)); err != nil {
			return err
		}
	} else if err := os.Remove(r.path); err != nil {
		return err
	}

	r.size = 0
	return r.open()
}

// backupPath names the nth rotated file: labdash.1.log, labdash.2.log. The
// extension stays last so that a file manager still knows what it is.
func (r *rotatingFile) backupPath(n int) string {
	ext := filepath.Ext(r.path)
	base := strings.TrimSuffix(r.path, ext)
	return fmt.Sprintf("%s.%d%s", base, n, ext)
}

// existingFiles lists the current file and its backups. `labdash doctor` will
// offer to attach them; until it exists, the rotation test reads this to check
// that the budget is honoured.
func (r *rotatingFile) existingFiles() []string {
	var out []string
	if _, err := os.Stat(r.path); err == nil {
		out = append(out, r.path)
	}
	for i := 1; i < r.keep; i++ {
		if _, err := os.Stat(r.backupPath(i)); err == nil {
			out = append(out, r.backupPath(i))
		}
	}
	sort.Strings(out)
	return out
}
