package gitlabauth

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// frozen names the four identifiers a user's machine remembers, and why each
// one may not move. The failure message is the whole point of the test: a
// contributor who trips it has to be told what they stranded.
var frozen = []struct {
	what, value, consequence string
}{
	{
		what:        "the keyring service",
		value:       keyringService,
		consequence: "every stored credential is stranded and every user silently appears logged out",
	},
	{
		what:        "the configuration directory name",
		value:       "labdash",
		consequence: "the settings file and the credential file are both looked for somewhere new, and the old ones are invisible",
	},
	{
		what:        "the credential file name",
		value:       credentialsFile,
		consequence: "every credential stored without a keyring is stranded — headless Linux, containers, most CI",
	},
	{
		what:        "the settings file name",
		value:       settingsFile,
		consequence: "a user's configured instances vanish with no error to explain it",
	},
}

// REG-13, AUT-11.T4 and CFG-17.T2 — the identifiers a user's machine
// remembers, asserted as literals.
//
// Prevents: a rename or a tidy-up stranding stored state. This already happened
// once, during the gitlab-tui to labdash rename. The assertions are
// deliberately brittle; that is their job.
func TestREG13_MachineRememberedIdentifiersAreFrozen(t *testing.T) {
	t.Parallel()

	want := []string{"com.gitlab.labdash", "labdash", "credentials.json", "settings.yml"}

	for i, f := range frozen {
		require.Equal(t, want[i], f.value,
			"%s changed from %q to %q.\n\nThis is frozen: %s.\n"+
				"Changing it needs a migration that moves the existing state first.",
			f.what, want[i], f.value, f.consequence)
	}
}

// CFG-17.T2 — the settings file resolves to a path ending in settings.yml, and
// the flag and variable a user already has in a script keep their names.
//
// Prevents: a REG-13-class break where the file is renamed and every existing
// install silently loses its instances.
func TestCFG17T2_TheSettingsFileIsSettingsYml(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LABDASH_CONFIG_DIR", dir)

	require.Equal(t, filepath.Join(dir, "settings.yml"), SettingsPath())
	require.True(t, strings.HasSuffix(SettingsPath(), "settings.yml"),
		"the settings file is named settings.yml, asserted as a literal")
}

// CFG-01.T1 and REG-09 — the per-platform location, with Windows named
// explicitly because that is where the landmine is.
//
// Prevents: writing the file to roaming %APPDATA%, where nothing reads it.
func TestCFG01T1_TheConfigDirectoryIsTheDocumentedOne(t *testing.T) {
	t.Setenv("LABDASH_CONFIG_DIR", "")

	dir := ConfigDir()
	require.Equal(t, "labdash", filepath.Base(dir),
		"our files live in a directory named labdash")

	if runtime.GOOS == "windows" {
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			t.Skip("LOCALAPPDATA is not set")
		}
		require.True(t, strings.HasPrefix(dir, local),
			"on Windows the config directory is under %%LOCALAPPDATA%% (%s), got %s", local, dir)
		require.NotContains(t, strings.ToLower(dir), "roaming",
			"os.UserConfigDir() returns roaming %%APPDATA%%; xdg.ConfigHome does not")
	}
}

// CFG-23.T2 — nothing in the auth path needs a settings file to exist.
//
// Prevents: writing a starter file nobody asked for. The file is created when
// the user or the wizard adds an instance, and only then.
func TestCFG23T2_TheAuthPathNeverCreatesASettingsFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LABDASH_CONFIG_DIR", dir)
	clearTokenEnv(t)

	cfg, err := LoadConfig()
	require.NoError(t, err, "a missing settings file is the normal state, not an error")
	require.Empty(t, cfg.Instances)

	// A whole failed resolution, which is the longest path through the auth
	// code that runs before anybody has logged in.
	_, err = Resolve(Options{Config: &cfg, Store: newTestStore(t)})
	require.ErrorIs(t, err, ErrNoToken)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries,
		"the auth path wrote something into a config directory that should still be empty")
}
