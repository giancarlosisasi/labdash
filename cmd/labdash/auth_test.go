package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/gitlabauth"
	"github.com/giancarlosisasi/labdash/internal/testsupport/fakegitlab"
)

// isolate gives a test its own config directory and its own credential store,
// so it can never read — or overwrite — the developer's real credential. The
// store forces the file backend: the OS keyring is one namespace shared with
// the machine's actual labdash install.
func isolate(t *testing.T) (string, deps) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("LABDASH_CONFIG_DIR", dir)
	for _, name := range []string{
		"GITLAB_TOKEN", "GITLAB_ACCESS_TOKEN", "OAUTH_TOKEN",
		"GITLAB_HOST", "GITLAB_URI", "GL_HOST", "LABDASH_CLIENT_ID",
	} {
		t.Setenv(name, "")
	}

	return dir, deps{Store: &gitlabauth.Store{Dir: dir, DisableKeyring: true}}
}

// run executes one labdash command and returns everything it printed.
func run(t *testing.T, d deps, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	cmd := newRootCmd(nil, d)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs(args)

	err := cmd.Execute()
	return out.String(), err
}

// AUT-05.T1 — what `auth status` reports.
//
// Prevents: leaking a credential into a support paste. The command exists to be
// pasted into a bug report, so the one thing it must never print is the value.
func TestAUT05T1_AuthStatusReportsEverythingButTheValue(t *testing.T) {
	dir, d := isolate(t)

	const secret = "glpat-this-must-never-be-printed"
	store := d.Store
	require.NoError(t, store.Save("gitlab.com", gitlabauth.StoredToken{
		AccessToken: secret, Kind: gitlabauth.KindPAT, Scopes: []string{"api"},
	}))

	out, err := run(t, d, "auth", "status", "--offline")
	require.NoError(t, err)

	require.NotContains(t, out, secret, "auth status printed the credential")
	for _, want := range []string{
		"gitlab.com",                      // which instance
		"hidden",                          // and that the value is deliberately absent
		filepath.Join(dir, "credentials"), // where it is stored
		"settings",                        // which settings file was read
		"api/graphql",                     // what it will talk to
	} {
		require.Contains(t, out, want, "auth status does not report %q", want)
	}
}

// AUT-05.T2 — `auth status --offline` makes no network call.
//
// Prevents: a status command that hangs on a VPN, which is exactly when
// somebody runs it.
func TestAUT05T2_AuthStatusOfflineMakesNoRequest(t *testing.T) {
	dir, d := isolate(t)

	srv := fakegitlab.New(t)
	host := strings.TrimPrefix(srv.URL, "http://")
	writeSettings(t, dir, "defaultHost: "+host+"\ninstances:\n  "+host+":\n    apiProtocol: http\n")

	store := d.Store
	require.NoError(t, store.Save(host, gitlabauth.StoredToken{
		AccessToken: "stored", Kind: gitlabauth.KindPAT,
	}))

	_, err := run(t, d, "auth", "status", "--offline")
	require.NoError(t, err)
	require.Empty(t, srv.Requests(), "--offline made a request anyway")
}

// AUT-06.T1 — `auth logout` with nothing stored.
//
// Prevents: the historical bug where logout claimed success and changed
// nothing, so a user who wanted to be logged out believed they were.
func TestAUT06T1_LogoutIsHonestWhenThereWasNothing(t *testing.T) {
	_, d := isolate(t)

	out, err := run(t, d, "auth", "logout", "--hostname", "gitlab.example.com")
	require.NoError(t, err)
	require.Contains(t, out, "nothing to remove")
	require.NotContains(t, out, "Removed the labdash credential",
		"logout claimed to remove a credential that was never there")

	store := d.Store
	require.NoError(t, store.Save("gitlab.example.com", gitlabauth.StoredToken{
		AccessToken: "stored", Kind: gitlabauth.KindPAT,
	}))

	out, err = run(t, d, "auth", "logout", "--hostname", "gitlab.example.com")
	require.NoError(t, err)
	require.Contains(t, out, "Removed the labdash credential")
	require.Contains(t, out, "stays valid at GitLab",
		"forgetting a token is not revoking it, and the difference matters")

	require.False(t, store.Has("gitlab.example.com"))
}

// AUT-03.T3 — the command never asks for a token at a terminal.
//
// Prevents: the un-cancellable prompt this replaced. Reading a secret with echo
// off puts the terminal into raw mode, and a raw-mode read swallows Ctrl+C, so
// the only way out was to kill the shell. Pasting a token by hand is the
// wizard's screen now; this command is the one a script uses.
func TestAUT03T3_TheCommandNeverPromptsForAToken(t *testing.T) {
	t.Parallel()

	_, err := readToken(strings.NewReader("glpat-secret\n"), "gitlab.example.com", true)
	require.Error(t, err, "a terminal must not be prompted at")

	msg := err.Error()
	require.NotContains(t, msg, "glpat-secret", "the token reached the error")
	require.Contains(t, msg, "labdash\n", "the refusal must name the app as the way to paste one")
	require.Contains(t, msg, "echo $TOKEN", "and the pipe, for a script")
	require.Contains(t, msg, "gitlab.example.com")
}

// CFG-17.T1 — `settings path` and `settings show`.
//
// Prevents: a screenshot leaking a token, and a first-time user meeting an
// error where an example would have helped.
func TestCFG17T1_SettingsPathAndShow(t *testing.T) {
	dir, d := isolate(t)

	t.Run("path prints the settings file", func(t *testing.T) {
		out, err := run(t, d, "settings", "path")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dir, "settings.yml"), strings.TrimSpace(out))
	})

	t.Run("show prints an example when nothing is configured", func(t *testing.T) {
		out, err := run(t, d, "settings", "show")
		require.NoError(t, err)
		require.Contains(t, out, "No instances configured")
		require.Contains(t, out, "tokenEnv:", "the example should show the recommended shape")
		require.NotContains(t, out, "Error")
	})

	t.Run("show prints the instances and never a credential", func(t *testing.T) {
		const secret = "glpat-must-not-appear-in-a-screenshot"
		writeSettings(t, dir, "instances:\n  gitlab.example.com:\n"+
			"    tokenEnv: WORK_TOKEN\n    caCert: /etc/ssl/corp.pem\n    insecureSkipVerify: true\n")
		t.Setenv("WORK_TOKEN", secret)

		store := d.Store
		require.NoError(t, store.Save("gitlab.example.com", gitlabauth.StoredToken{
			AccessToken: secret, Kind: gitlabauth.KindPAT,
		}))

		out, err := run(t, d, "settings", "show")
		require.NoError(t, err)

		require.Contains(t, out, "gitlab.example.com")
		require.Contains(t, out, "WORK_TOKEN", "the file records the variable's name")
		require.NotContains(t, out, secret, "settings show printed a credential")
		require.Contains(t, out, "TLS verification is OFF",
			"a host with verification off should say so out loud")
	})
}

func writeSettings(t *testing.T, dir, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.yml"), []byte(body), 0o600))
}
