package gitlabauth

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

// recordingKeyring is an in-memory keyring that remembers every service string
// it was asked about. Watching the calls is what makes REG-10 an assertion
// rather than a code review: a grep for "glab" keeps passing after somebody
// renames a variable.
type recordingKeyring struct {
	mu      sync.Mutex
	asked   []string
	entries map[string]string
}

func newRecordingKeyring() *recordingKeyring {
	return &recordingKeyring{entries: map[string]string{}}
}

func (k *recordingKeyring) note(service string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.asked = append(k.asked, service)
}

func (k *recordingKeyring) services() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return slices.Clone(k.asked)
}

func (k *recordingKeyring) Get(service, _ string) (string, error) {
	k.note(service)
	k.mu.Lock()
	defer k.mu.Unlock()
	if v, ok := k.entries[service]; ok {
		return v, nil
	}
	return "", keyring.ErrNotFound
}

func (k *recordingKeyring) Set(service, _, password string) error {
	k.note(service)
	k.mu.Lock()
	defer k.mu.Unlock()
	k.entries[service] = password
	return nil
}

func (k *recordingKeyring) Delete(service, _ string) error {
	k.note(service)
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.entries[service]; !ok {
		return keyring.ErrNotFound
	}
	delete(k.entries, service)
	return nil
}

// AUT-11.T1 — a credential round-trips through the keyring backend.
//
// Prevents: the keyring path being unverified. Only the file fallback was
// covered before, and the keyring is what almost every user actually gets.
func TestAUT11T1_TheKeyringRoundTripsACredential(t *testing.T) {
	t.Parallel()

	ring := newRecordingKeyring()
	store := &Store{Dir: t.TempDir(), Keyring: ring}

	want := StoredToken{AccessToken: "kept", RefreshToken: "renewable", Kind: KindOAuth}
	require.NoError(t, store.Save("gitlab.example.com", want))

	got, err := store.Load("gitlab.example.com")
	require.NoError(t, err)
	require.Equal(t, want.AccessToken, got.AccessToken)
	require.Equal(t, want.RefreshToken, got.RefreshToken)

	require.NoFileExists(t, store.filePath(),
		"a keyring write must not also leave a copy on disk")

	removed, err := store.Delete("gitlab.example.com")
	require.NoError(t, err)
	require.True(t, removed)
}

// AUT-11.T4 — the keyring service string, asserted where it is used.
//
// Prevents: a rename stranding every stored credential. REG-13 freezes the
// constant; this freezes the key we actually ask the OS for.
func TestAUT11T4_TheKeyringKeyIsTheNamespacedService(t *testing.T) {
	t.Parallel()

	ring := newRecordingKeyring()
	store := &Store{Dir: t.TempDir(), Keyring: ring}
	require.NoError(t, store.Save("gitlab.com", StoredToken{AccessToken: "x", Kind: KindPAT}))

	require.Equal(t, []string{"com.gitlab.labdash:gitlab.com"}, ring.services(),
		"the keyring entry must be exactly com.gitlab.labdash:<host>")
}

// AUT-11.T2 — no keyring backend available.
//
// Prevents: a crash mid-write truncating the credential file. It is written to
// a temp file in the same directory and renamed, so the file a reader sees is
// always a whole one.
func TestAUT11T2_WithoutAKeyringTheFileIsWrittenAndReplacedWhole(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := &Store{Dir: dir, DisableKeyring: true}

	require.NoError(t, store.Save("gitlab.com", StoredToken{AccessToken: "first", Kind: KindPAT}))
	require.NoError(t, store.Save("gitlab.com", StoredToken{AccessToken: "second", Kind: KindPAT}))

	got, err := store.Load("gitlab.com")
	require.NoError(t, err)
	require.Equal(t, "second", got.AccessToken)

	// A rename that left its temp file behind would show up here, and a stray
	// half-written credential file is exactly what this is guarding.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, credentialsFile, entries[0].Name())
}

// AUT-11.T3 — the file fallback restricts the credential to its owner.
//
// Prevents: silently doing nothing about permissions, which is where glab gives
// up on Windows. On Unix that is mode 0600; on Windows the file inherits the
// per-user ACL of %LOCALAPPDATA%, which is why the mode bits are not asserted
// there.
func TestAUT11T3_TheCredentialFileIsOwnerOnly(t *testing.T) {
	t.Parallel()

	store := &Store{Dir: t.TempDir(), DisableKeyring: true}
	require.NoError(t, store.Save("gitlab.com", StoredToken{AccessToken: "x", Kind: KindPAT}))

	info, err := os.Stat(store.filePath())
	require.NoError(t, err)

	if runtime.GOOS != "windows" {
		require.Equal(t, fs.FileMode(0o600), info.Mode().Perm(),
			"the credential file must be readable only by its owner")
	}
	require.NotZero(t, info.Size())
}

// REG-10 — labdash never reads or writes glab's configuration or keyring.
//
// Prevents: the one unforgivable failure. glab mints OAuth tokens GitLab
// expires after two hours and rotates the refresh token on use, so renewing a
// borrowed credential would break the user's glab. This is asserted by
// observation: glab's credential is planted in both of its homes, and labdash
// is watched failing to find anything.
func TestREG10_GlabsCredentialsAreNeverTouched(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LABDASH_CONFIG_DIR", filepath.Join(root, "labdash"))
	clearTokenEnv(t)

	// glab's config, in the shape and the place glab keeps it.
	glabDir := filepath.Join(root, "glab-cli")
	require.NoError(t, os.MkdirAll(glabDir, 0o700))
	glabConfig := filepath.Join(glabDir, "config.yml")
	glabBody := []byte("hosts:\n  gitlab.com:\n    token: glab-owns-this\n")
	require.NoError(t, os.WriteFile(glabConfig, glabBody, 0o600))

	// glab's keyring entries, under glab's own service prefix.
	ring := newRecordingKeyring()
	require.NoError(t, ring.Set("glab:gitlab.com", "", "glab-owns-this-too"))
	require.NoError(t, ring.Set("glab", "", "glab-owns-this-as-well"))
	before := ring.services()

	store := &Store{Dir: filepath.Join(root, "labdash"), Keyring: ring}

	_, err := Resolve(Options{Config: &Config{}, Store: store})
	require.ErrorIs(t, err, ErrNoToken,
		"labdash found a credential, and the only one on this machine is glab's")

	// Nothing we asked the keyring for may be glab's.
	for _, service := range ring.services()[len(before):] {
		require.NotContains(t, strings.ToLower(service), "glab:",
			"labdash asked the keyring for %q, which is glab's namespace", service)
		require.Equal(t, keyringService+":", service[:len(keyringService)+1],
			"every service string labdash uses starts with %s:", keyringService)
	}

	// glab's file is byte-identical, so we neither read it into a credential
	// nor rewrote it.
	after, err := os.ReadFile(glabConfig)
	require.NoError(t, err)
	require.Equal(t, glabBody, after, "glab's configuration was modified")

	// And nothing labdash names as a search location is inside glab's tree.
	for _, path := range []string{SettingsPath(), store.filePath(), store.Describe("gitlab.com")} {
		require.NotContains(t, filepath.ToSlash(path), "glab-cli",
			"labdash searches %q, which is glab's directory", path)
	}
}

// REG-10, second half — a keyring that is simply not there degrades to the file
// backend instead of failing the command.
//
// Prevents: headless Linux and most containers being unable to log in at all.
func TestREG10_AnAbsentKeyringDegradesToTheFile(t *testing.T) {
	t.Parallel()

	store := &Store{Dir: t.TempDir(), Keyring: unavailableKeyring{}}
	require.NoError(t, store.Save("gitlab.com", StoredToken{AccessToken: "x", Kind: KindPAT}))

	got, err := store.Load("gitlab.com")
	require.NoError(t, err)
	require.Equal(t, "x", got.AccessToken)
	require.FileExists(t, store.filePath())
}

// unavailableKeyring is a platform with no Secret Service running.
type unavailableKeyring struct{}

func (unavailableKeyring) Get(string, string) (string, error) {
	return "", keyring.ErrUnsupportedPlatform
}
func (unavailableKeyring) Set(string, string, string) error { return errors.New("no keyring here") }
func (unavailableKeyring) Delete(string, string) error      { return keyring.ErrUnsupportedPlatform }
