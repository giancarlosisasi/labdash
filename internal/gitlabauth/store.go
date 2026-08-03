package gitlabauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"github.com/zalando/go-keyring"
)

// keyringService is the prefix for our own keyring entries. It is deliberately
// distinct from glab's "glab:" prefix: we own our credentials, glab owns its
// own, and neither tool ever rotates the other's refresh token.
//
// It is reverse-DNS rather than a bare "labdash" because an OS keyring is a
// single flat namespace shared by every program on the machine. Two programs
// choosing the same service string get the same entry — the second write wins
// and the first program's credential is gone. "com.gitlab.labdash" is derived
// from the gitlab.com/labdash group we own, so no other project can pick it
// without squatting a namespace that is verifiably ours.
//
// Changing this value strands every stored credential and forces every user to
// log in again. Do not change it without a migration plan.
const keyringService = "com.gitlab.labdash"

// credentialsFile is the fallback when no OS keyring backend is available —
// headless Linux, most containers, some CI images.
const credentialsFile = "credentials.json"

// ErrNoStoredToken means this host has never been logged in to.
var ErrNoStoredToken = errors.New("no stored credential for host")

// TokenKind distinguishes what we are holding, because only OAuth tokens can
// be refreshed and only OAuth tokens expire in two hours.
type TokenKind string

const (
	KindOAuth TokenKind = "oauth"
	KindPAT   TokenKind = "pat"
)

// StoredToken is our own on-disk credential record. The JSON field names match
// golang.org/x/oauth2's Token so the two convert without surprises.
type StoredToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
	Kind         TokenKind `json:"kind"`
}

// String redacts both secrets, so a StoredToken cannot leak through %v.
func (t StoredToken) String() string {
	return fmt.Sprintf("StoredToken{Kind:%s AccessToken:<%d chars, redacted> RefreshToken:<%d chars, redacted> Expiry:%s}",
		t.Kind, len(t.AccessToken), len(t.RefreshToken), t.Expiry.Format(time.RFC3339))
}

// Store persists credentials we own. It prefers the OS keyring and falls back
// to a 0600 file when no keyring backend is reachable.
type Store struct {
	// Dir is our config directory. Empty means the platform default.
	Dir string
	// DisableKeyring forces the file backend. For tests.
	DisableKeyring bool
}

// NewStore returns a Store using the platform config directory, overridable
// with LABDASH_CONFIG_DIR.
func NewStore() *Store {
	return &Store{Dir: ConfigDir()}
}

// ConfigDir is where labdash keeps its own files: XDG config home on Linux,
// ~/Library/Application Support on macOS, %LOCALAPPDATA% on Windows. Override
// with LABDASH_CONFIG_DIR.
func ConfigDir() string {
	if dir := os.Getenv("LABDASH_CONFIG_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(xdg.ConfigHome, "labdash")
}

func (s *Store) dir() string {
	if s.Dir != "" {
		return s.Dir
	}
	return ConfigDir()
}

func (s *Store) keyringKey(host string) string {
	return keyringService + ":" + host
}

func (s *Store) filePath() string {
	return filepath.Join(s.dir(), credentialsFile)
}

// Location describes, in one line, where this host's credential is kept. Safe
// to print: it names a store, never a value.
func (s *Store) Location(host string) string {
	if !s.DisableKeyring {
		if _, err := keyring.Get(s.keyringKey(host), ""); err == nil {
			return "OS keyring (" + s.keyringKey(host) + ")"
		}
	}
	return s.filePath()
}

// Load returns the stored credential for host, or ErrNoStoredToken.
func (s *Store) Load(host string) (StoredToken, error) {
	if !s.DisableKeyring {
		raw, err := keyring.Get(s.keyringKey(host), "")
		if err == nil {
			tok := StoredToken{}
			if err := json.Unmarshal([]byte(raw), &tok); err != nil {
				return StoredToken{}, fmt.Errorf("decoding keyring credential for %s: %w", host, err)
			}
			return tok, nil
		}
		if !errors.Is(err, keyring.ErrNotFound) && !isKeyringUnavailable(err) {
			return StoredToken{}, fmt.Errorf("reading keyring for %s: %w", host, err)
		}
	}

	all, err := s.readFile()
	if err != nil {
		return StoredToken{}, err
	}

	tok, ok := all[host]
	if !ok {
		return StoredToken{}, fmt.Errorf("%w: %s", ErrNoStoredToken, host)
	}

	return tok, nil
}

// Save writes the credential for host, preferring the OS keyring.
func (s *Store) Save(host string, tok StoredToken) error {
	if !s.DisableKeyring {
		raw, err := json.Marshal(tok)
		if err != nil {
			return fmt.Errorf("encoding credential: %w", err)
		}
		if err := keyring.Set(s.keyringKey(host), "", string(raw)); err == nil {
			// Clear any older file copy so one credential does not shadow the other.
			_, _ = s.deleteFromFile(host)
			return nil
		}
	}

	return s.saveToFile(host, tok)
}

// Delete removes the credential for host from both backends and reports
// whether anything was actually there. Callers must not claim to have removed
// something that never existed.
func (s *Store) Delete(host string) (removed bool, err error) {
	if !s.DisableKeyring {
		if err := keyring.Delete(s.keyringKey(host), ""); err == nil {
			removed = true
		}
	}

	removedFromFile, err := s.deleteFromFile(host)
	if err != nil {
		return removed, err
	}

	return removed || removedFromFile, nil
}

// Has reports whether we hold a credential for host, without decoding or
// returning it.
func (s *Store) Has(host string) bool {
	_, err := s.Load(host)
	return err == nil
}

// Describe names the backends that were searched for host, without revealing
// whether one matched. Safe to print in a "not logged in" message.
func (s *Store) Describe(host string) string {
	if s.DisableKeyring {
		return s.filePath()
	}
	return fmt.Sprintf("OS keyring (%s), then %s", s.keyringKey(host), s.filePath())
}

// Hosts lists every host we hold a credential for.
func (s *Store) Hosts() []string {
	all, err := s.readFile()
	if err != nil {
		return nil
	}

	hosts := make([]string, 0, len(all))
	for h := range all {
		hosts = append(hosts, h)
	}

	return hosts
}

func (s *Store) readFile() (map[string]StoredToken, error) {
	raw, err := os.ReadFile(s.filePath())
	if errors.Is(err, os.ErrNotExist) {
		return map[string]StoredToken{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", s.filePath(), err)
	}

	all := map[string]StoredToken{}
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", s.filePath(), err)
	}

	return all, nil
}

func (s *Store) writeFile(all map[string]StoredToken) error {
	if err := os.MkdirAll(s.dir(), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", s.dir(), err)
	}

	raw, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding credentials: %w", err)
	}

	// Write to a temp file in the same directory, then rename, so a crash
	// mid-write cannot leave a truncated credential file behind.
	tmp, err := os.CreateTemp(s.dir(), credentialsFile+".*")
	if err != nil {
		return fmt.Errorf("creating temp credentials file: %w", err)
	}
	tmpName := tmp.Name()

	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if err := tmp.Chmod(0o600); err != nil && !errors.Is(err, os.ErrInvalid) {
		// Chmod is a no-op on Windows; only Unix failures matter.
		return fmt.Errorf("securing temp credentials file: %w", err)
	}
	if _, err := tmp.Write(raw); err != nil {
		return fmt.Errorf("writing temp credentials file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp credentials file: %w", err)
	}

	if err := os.Rename(tmpName, s.filePath()); err != nil {
		return fmt.Errorf("replacing %s: %w", s.filePath(), err)
	}

	return os.Chmod(s.filePath(), 0o600)
}

func (s *Store) saveToFile(host string, tok StoredToken) error {
	all, err := s.readFile()
	if err != nil {
		return err
	}
	all[host] = tok

	return s.writeFile(all)
}

func (s *Store) deleteFromFile(host string) (removed bool, err error) {
	all, err := s.readFile()
	if err != nil {
		return false, err
	}
	if _, ok := all[host]; !ok {
		return false, nil
	}
	delete(all, host)

	return true, s.writeFile(all)
}

// isKeyringUnavailable reports whether the error means "no keyring backend
// here" rather than "this entry is missing". Headless Linux and most
// containers have no Secret Service, and that must degrade to the file
// backend rather than failing the command.
func isKeyringUnavailable(err error) bool {
	return errors.Is(err, keyring.ErrUnsupportedPlatform)
}
