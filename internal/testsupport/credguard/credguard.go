// Package credguard is REG-06: the suite-wide check that nothing labdash's
// tests produce contains a credential.
//
// It scans test output, golden files, fixtures and log files for the shapes
// GitLab issues and for the shapes an HTTP client writes. A match fails the
// package that produced it.
//
// This is a regression, not a precaution. A test leaked a real token into its
// output on 2026-08-01 (research/12-current-state.md §9). The guard exists so
// the repeat is impossible rather than unlikely, and it is in the first change
// because a guard added later never covers what came before it.
package credguard

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// A pattern is one credential shape, with the name a failure reports.
type pattern struct {
	name string
	re   *regexp.Regexp
}

// patterns is the whole vocabulary of credential shapes. Each is anchored on a
// prefix or a header name rather than on entropy, because an entropy heuristic
// fires on git hashes and base64 fixtures and is then switched off by whoever
// is annoyed by it.
var patterns = []pattern{
	// GitLab's own prefixed tokens. The prefix is fixed; the body is at least
	// twenty characters of the URL-safe alphabet.
	{"personal access token (glpat-)", regexp.MustCompile(`glpat-[A-Za-z0-9_\-]{20,}`)},
	{"deploy token (gldt-)", regexp.MustCompile(`gldt-[A-Za-z0-9_\-]{20,}`)},
	{"runner token (glrt-)", regexp.MustCompile(`glrt-[A-Za-z0-9_\-]{20,}`)},
	{"CI job token (glcbt-)", regexp.MustCompile(`glcbt-[A-Za-z0-9_\-]{20,}`)},
	{"feed token (glft-)", regexp.MustCompile(`glft-[A-Za-z0-9_\-]{20,}`)},

	// OAuth. GitLab's access and refresh tokens are 64 hex characters, which is
	// also what an Authorization header carries once the flow completes.
	{"64-character hex token", regexp.MustCompile(`\b[0-9a-f]{64}\b`)},
	{"Bearer credential", regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/\-]{16,}=*`)},

	// GitLab's own header, whatever it happens to carry.
	{"PRIVATE-TOKEN header", regexp.MustCompile(`(?i)PRIVATE-TOKEN\s*[:=]\s*\S+`)},
	{"JOB-TOKEN header", regexp.MustCompile(`(?i)JOB-TOKEN\s*[:=]\s*\S+`)},
}

// allowed are strings that match a pattern but are provably not a credential.
// Every entry needs a reason, because an allow-list is how a guard is disabled
// one entry at a time.
var allowed = []string{
	// The redaction the HTTP logger writes in place of a token (DIA-02).
	"Bearer [redacted]",
	"PRIVATE-TOKEN: [redacted]",
	"JOB-TOKEN: [redacted]",
	// The literal patterns above, as they appear in this file and in the tests
	// that prove the guard fires.
	"glpat-EXAMPLE",
	"[0-9a-f]{64}",
}

// A Finding is one credential-shaped string, located.
type Finding struct {
	// Source names where it was found: a file path, or "stdout"/"stderr".
	Source string
	// Line is 1-indexed within Source, or 0 when the source is not line-based.
	Line int
	// Pattern is the human name of the shape that matched.
	Pattern string
	// Excerpt is the surrounding line with the credential body replaced. The
	// guard never prints the value it found, because a failing CI log is public.
	Excerpt string
}

func (f Finding) String() string {
	loc := f.Source
	if f.Line > 0 {
		loc = fmt.Sprintf("%s:%d", f.Source, f.Line)
	}
	return fmt.Sprintf("%s: %s\n      %s", loc, f.Pattern, f.Excerpt)
}

// Scan reports every credential shape in b, attributing findings to source.
func Scan(source string, b []byte) []Finding {
	var out []Finding
	for i, line := range strings.Split(string(b), "\n") {
		out = append(out, scanLine(source, i+1, line)...)
	}
	return out
}

func scanLine(source string, n int, line string) []Finding {
	if line == "" {
		return nil
	}
	var out []Finding
	for _, p := range patterns {
		loc := p.re.FindStringIndex(line)
		if loc == nil {
			continue
		}
		if isAllowed(line[loc[0]:loc[1]]) {
			continue
		}
		out = append(out, Finding{
			Source:  source,
			Line:    n,
			Pattern: p.name,
			Excerpt: redact(line, loc),
		})
	}
	return out
}

func isAllowed(match string) bool {
	for _, a := range allowed {
		if strings.Contains(match, a) {
			return true
		}
	}
	return false
}

// redact replaces the matched span with its length, so a failure says where the
// leak is without repeating it into a build log.
func redact(line string, loc []int) string {
	head, tail := line[:loc[0]], line[loc[1]:]
	if len(head) > 40 {
		head = "…" + head[len(head)-40:]
	}
	if len(tail) > 40 {
		tail = tail[:40] + "…"
	}
	return fmt.Sprintf("%s<%d credential-shaped characters>%s", head, loc[1]-loc[0], tail)
}

// scanExtensions are the files worth reading. Everything a test writes lands in
// one of these; a binary artifact does not.
var scanExtensions = map[string]bool{
	".golden": true, ".json": true, ".yml": true, ".yaml": true,
	".txt": true, ".log": true, ".md": true, ".graphql": true, "": true,
}

// ScanDir walks root and scans every file a test could have written. A missing
// root is not an error: a package with no testdata has nothing to leak.
func ScanDir(root string) ([]Finding, error) {
	var out []Finding

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || !scanExtensions[filepath.Ext(path)] {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out = append(out, Scan(filepath.ToSlash(path), b)...)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return out, err
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

// A StreamScanner tees a stream through to its real destination while checking
// every line. It is how the guard reaches test output, which is where the 2026
// leak actually happened — not a file.
type StreamScanner struct {
	source string

	mu       sync.Mutex
	findings []Finding
}

// NewStreamScanner returns a scanner attributing findings to source.
func NewStreamScanner(source string) *StreamScanner {
	return &StreamScanner{source: source}
}

// Copy reads from r, writes every byte to w unchanged, and scans as it goes.
// It returns when r reaches EOF.
func (s *StreamScanner) Copy(w io.Writer, r io.Reader) {
	br := bufio.NewReader(r)
	n := 0
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			n++
			if _, werr := io.WriteString(w, line); werr != nil {
				return
			}
			if f := scanLine(s.source, n, strings.TrimRight(line, "\r\n")); f != nil {
				s.mu.Lock()
				s.findings = append(s.findings, f...)
				s.mu.Unlock()
			}
		}
		if err != nil {
			return
		}
	}
}

// Findings returns what the stream carried.
func (s *StreamScanner) Findings() []Finding {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Finding(nil), s.findings...)
}

// Report renders findings as the message a failing build prints.
func Report(findings []Finding) string {
	if len(findings) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "REG-06: %d credential-shaped string(s) found in what the tests produced.\n",
		len(findings))
	b.WriteString("A token must never reach test output, a golden file, a fixture or a log.\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "  %s\n", f)
	}
	return b.String()
}
