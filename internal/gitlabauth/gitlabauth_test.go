package gitlabauth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeOurConfig drops a labdash settings.yml into a temp dir and returns its path.
func writeOurConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "settings.yml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	return path
}

// clearTokenEnv removes every variable Resolve consults, so a developer's own
// exported token cannot make a test pass or fail spuriously.
func clearTokenEnv(t *testing.T) {
	t.Helper()

	for _, n := range append(append([]string{}, tokenEnvVars...), hostEnvVars...) {
		t.Setenv(n, "")
	}
}

func TestLoadConfig(t *testing.T) {
	path := writeOurConfig(t, `
defaultHost: gitlab.example.com
instances:
  gitlab.example.com:
    apiHost: api.example.com
    apiProtocol: https
    subfolder: gitlab
    clientId: abc123
    tokenEnv: WORK_TOKEN
    caCert: /etc/ssl/corp.pem
    insecureSkipVerify: true
    customHeaders:
      - name: Cf-Access-Client-Secret
        valueFromEnv: CF_SECRET
      - name: X-Static
        value: literal
`)

	cfg, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadConfigFrom: %v", err)
	}

	if cfg.DefaultHost != "gitlab.example.com" {
		t.Errorf("DefaultHost = %q", cfg.DefaultHost)
	}

	inst, ok := cfg.Instance("gitlab.example.com")
	if !ok {
		t.Fatal("instance not found")
	}
	if inst.APIHost != "api.example.com" || inst.Subfolder != "gitlab" {
		t.Errorf("instance = %+v", inst)
	}
	if inst.ClientID != "abc123" || inst.TokenEnv != "WORK_TOKEN" {
		t.Errorf("instance = %+v", inst)
	}
	if !inst.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
	if len(inst.CustomHeaders) != 2 {
		t.Fatalf("got %d custom headers, want 2", len(inst.CustomHeaders))
	}
}

// A missing config is the normal state for a gitlab.com user, not an error.
func TestLoadConfigMissingFileIsNotAnError(t *testing.T) {
	cfg, err := LoadConfigFrom(filepath.Join(t.TempDir(), "nope.yml"))
	if err != nil {
		t.Fatalf("a missing config must not be an error, got: %v", err)
	}
	if cfg.Instances == nil {
		t.Error("Instances must be initialised so callers can index it safely")
	}
}

func TestResolveHost(t *testing.T) {
	clearTokenEnv(t)

	tests := []struct {
		name     string
		cfg      Config
		explicit string
		envHost  string
		want     string
	}{
		{name: "nothing set", want: DefaultHost},
		{name: "explicit wins", cfg: Config{DefaultHost: "a.example.com"}, explicit: "b.example.com", want: "b.example.com"},
		{name: "config default", cfg: Config{DefaultHost: "a.example.com"}, want: "a.example.com"},
		{name: "env when no config", envHost: "c.example.com", want: "c.example.com"},
		{name: "config beats env", cfg: Config{DefaultHost: "a.example.com"}, envHost: "c.example.com", want: "a.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITLAB_HOST", tt.envHost)
			if got := tt.cfg.ResolveHost(tt.explicit); got != tt.want {
				t.Errorf("ResolveHost(%q) = %q, want %q", tt.explicit, got, tt.want)
			}
		})
	}
}

func TestResolveFromOurStore(t *testing.T) {
	clearTokenEnv(t)

	cfg := Config{Instances: map[string]InstanceConfig{
		"gitlab.example.com": {
			APIHost:            "api.example.com",
			Subfolder:          "gitlab",
			CACert:             "/etc/ssl/corp.pem",
			InsecureSkipVerify: true,
		},
	}}
	store := newTestStore(t)
	if err := store.Save("gitlab.example.com", StoredToken{
		AccessToken: "ours", Kind: KindOAuth, Expiry: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Resolve(Options{Host: "gitlab.example.com", Config: &cfg, Store: store})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if got.Token != "ours" {
		t.Errorf("token mismatch: got %d chars, want our stored credential", len(got.Token))
	}
	if !got.Managed {
		t.Error("a credential from our store must be Managed")
	}
	if want := "https://api.example.com/gitlab/api/graphql"; got.GraphQLEndpoint() != want {
		t.Errorf("GraphQLEndpoint() = %q, want %q — instance settings were dropped",
			got.GraphQLEndpoint(), want)
	}
	if got.CACert != "/etc/ssl/corp.pem" || !got.SkipTLSVerify {
		t.Errorf("TLS settings were dropped: %+v", got)
	}
}

// A per-instance tokenEnv is the most explicit signal available, so it beats
// even our own stored credential.
func TestResolveInstanceTokenEnvWins(t *testing.T) {
	clearTokenEnv(t)
	t.Setenv("WORK_TOKEN", "from-instance-env")

	cfg := Config{Instances: map[string]InstanceConfig{
		"gitlab.example.com": {TokenEnv: "WORK_TOKEN"},
	}}
	store := newTestStore(t)
	if err := store.Save("gitlab.example.com", StoredToken{AccessToken: "ours", Kind: KindOAuth}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Resolve(Options{Host: "gitlab.example.com", Config: &cfg, Store: store})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Token != "from-instance-env" {
		t.Errorf("token mismatch: got %d chars, want the instance's tokenEnv", len(got.Token))
	}
	if got.Managed {
		t.Error("a token from an environment variable is not ours to renew")
	}
	if got.Kind != KindPAT {
		t.Errorf("Kind = %q, want %q", got.Kind, KindPAT)
	}
}

// The generic variables are not host-scoped, so they must apply only to the
// default host. Otherwise one instance's token leaks to another.
func TestResolveGenericEnvIsDefaultHostOnly(t *testing.T) {
	clearTokenEnv(t)
	t.Setenv("GITLAB_TOKEN", "generic-env-token")

	cfg := Config{Instances: map[string]InstanceConfig{}}
	store := newTestStore(t)
	if err := store.Save("other.example.com", StoredToken{AccessToken: "ours-other", Kind: KindOAuth}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	t.Run("default host uses it", func(t *testing.T) {
		got, err := Resolve(Options{Config: &cfg, Store: store})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if got.Token != "generic-env-token" {
			t.Errorf("token mismatch: got %d chars, want the environment's", len(got.Token))
		}
	})

	t.Run("named host does not", func(t *testing.T) {
		got, err := Resolve(Options{Host: "other.example.com", Config: &cfg, Store: store})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if got.Token != "ours-other" {
			t.Errorf("token mismatch: got %d chars, want the stored one; "+
				"a generic variable must not reach a named host", len(got.Token))
		}
	})
}

func TestResolveAlternateEnvVars(t *testing.T) {
	for _, name := range []string{"GITLAB_ACCESS_TOKEN", "OAUTH_TOKEN"} {
		t.Run(name, func(t *testing.T) {
			clearTokenEnv(t)
			t.Setenv(name, "token-"+name)

			cfg := Config{}
			got, err := Resolve(Options{Config: &cfg, Store: newTestStore(t)})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got.Token != "token-"+name {
				t.Errorf("token mismatch: got %d chars, want token-%s", len(got.Token), name)
			}
		})
	}
}

func TestResolveNotLoggedInIsActionable(t *testing.T) {
	clearTokenEnv(t)

	cfg := Config{}
	_, err := Resolve(Options{
		Host:   "nowhere.example.com",
		Config: &cfg,
		Store:  newTestStore(t),
	})
	if err == nil {
		t.Fatal("Resolve succeeded with no credential available")
	}
	if !errors.Is(err, ErrNoToken) {
		t.Errorf("error does not unwrap to ErrNoToken: %v", err)
	}

	msg := err.Error()
	for _, want := range []string{
		"labdash auth login",
		"--with-token",
		"nowhere.example.com",
		"personal_access_tokens",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message is missing %q:\n%s", want, msg)
		}
	}
	// The old design told users to run glab. It no longer should.
	if strings.Contains(strings.ToLower(msg), "glab") {
		t.Errorf("the not-logged-in message still mentions glab:\n%s", msg)
	}
}

// A token must never reach a log line through fmt.
func TestCredentialsRedactSecrets(t *testing.T) {
	c := Credentials{
		Host:         "gitlab.com",
		Token:        "super-secret-access",
		RefreshToken: "super-secret-refresh",
		Source:       "test",
	}

	for _, s := range []string{c.String(), c.Describe()} {
		for _, secret := range []string{"super-secret-access", "super-secret-refresh"} {
			if strings.Contains(s, secret) {
				t.Errorf("%s leaked into output: %s", secret, s)
			}
		}
	}
	if !strings.Contains(c.String(), "redacted") {
		t.Errorf("String() should say the secrets are redacted, got %s", c.String())
	}
}

func TestConfigDirIsPlatformCorrect(t *testing.T) {
	t.Run("LABDASH_CONFIG_DIR overrides", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("LABDASH_CONFIG_DIR", dir)

		if got := ConfigDir(); got != dir {
			t.Errorf("ConfigDir() = %q, want %q", got, dir)
		}
		if got := SettingsPath(); got != filepath.Join(dir, "settings.yml") {
			t.Errorf("SettingsPath() = %q", got)
		}
	})

	t.Run("platform default is not roaming AppData", func(t *testing.T) {
		t.Setenv("LABDASH_CONFIG_DIR", "")

		dir := ConfigDir()
		if !strings.Contains(dir, "labdash") {
			t.Errorf("ConfigDir() = %q, want it under a labdash directory", dir)
		}

		// os.UserConfigDir() returns roaming %APPDATA% on Windows, which is not
		// where xdg.ConfigHome points. Guard against a refactor swapping them.
		if runtime.GOOS == "windows" {
			local := os.Getenv("LOCALAPPDATA")
			if local == "" {
				t.Skip("LOCALAPPDATA not set")
			}
			if !strings.HasPrefix(dir, local) {
				t.Errorf("ConfigDir() = %q, want it under %%LOCALAPPDATA%% (%s)", dir, local)
			}
		}
	})
}

func TestHeaderResolve(t *testing.T) {
	t.Setenv("MY_SECRET_VALUE", "from-env")

	tests := []struct {
		name string
		h    Header
		want string
	}{
		{name: "literal", h: Header{Name: "X-A", Value: "literal"}, want: "literal"},
		{name: "from env", h: Header{Name: "X-B", ValueFromEnv: "MY_SECRET_VALUE"}, want: "from-env"},
		{name: "missing env", h: Header{Name: "X-C", ValueFromEnv: "NOT_SET_ANYWHERE"}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.h.Resolve(); got != tt.want {
				t.Errorf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}
