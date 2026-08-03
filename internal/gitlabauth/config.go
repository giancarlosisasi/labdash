package gitlabauth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// settingsFile is labdash's own settings file, kept beside our credentials.
// We do not read glab's config: borrowing another tool's credentials meant
// inheriting its expiry and its refresh chain, and left us unable to renew
// anything. See research/09-auth-strategy.md.
const settingsFile = "settings.yml"

// Config describes the GitLab instances labdash talks to.
//
// It holds no secrets. Credentials live in the OS keyring, or in
// credentials.json at mode 0600 when no keyring is available.
type Config struct {
	// DefaultHost is used when no --hostname is given. Empty means gitlab.com,
	// unless $GITLAB_HOST says otherwise.
	DefaultHost string `yaml:"defaultHost,omitempty"`
	// Instances maps a host to its settings. A host with no entry still works;
	// it just gets the defaults.
	Instances map[string]InstanceConfig `yaml:"instances,omitempty"`
}

// InstanceConfig is everything we need to reach one GitLab instance, beyond
// the credential itself.
type InstanceConfig struct {
	// APIHost is the hostname API calls go to when it differs from the map
	// key — instances behind a proxy or an SSH alias.
	APIHost string `yaml:"apiHost,omitempty"`
	// APIProtocol is "https" (default) or, rarely, "http".
	APIProtocol string `yaml:"apiProtocol,omitempty"`
	// Subfolder is set when GitLab is installed under a path rather than at the
	// domain root, e.g. "gitlab" for https://example.com/gitlab/.
	Subfolder string `yaml:"subfolder,omitempty"`

	// ClientID is the OAuth application ID for this instance. Required for
	// self-managed, which cannot use the gitlab.com application.
	ClientID string `yaml:"clientId,omitempty"`
	// TokenEnv names an environment variable holding a personal access token
	// for this instance. This is the recommended way to keep a token out of
	// any file while still supporting several instances at once.
	TokenEnv string `yaml:"tokenEnv,omitempty"`

	// TLS and transport.
	CACert             string   `yaml:"caCert,omitempty"`
	ClientCert         string   `yaml:"clientCert,omitempty"`
	ClientKey          string   `yaml:"clientKey,omitempty"`
	InsecureSkipVerify bool     `yaml:"insecureSkipVerify,omitempty"`
	Proxy              string   `yaml:"proxy,omitempty"`
	CustomHeaders      []Header `yaml:"customHeaders,omitempty"`
}

// Header is one entry of an instance's customHeaders list. Auth proxies such
// as Cloudflare Access need these on every request.
type Header struct {
	Name         string `yaml:"name"`
	Value        string `yaml:"value,omitempty"`
	ValueFromEnv string `yaml:"valueFromEnv,omitempty"`
}

// Resolve returns the header value, reading the environment when the config
// pointed at a variable rather than a literal.
func (h Header) Resolve() string {
	if h.ValueFromEnv != "" {
		return os.Getenv(h.ValueFromEnv)
	}
	return h.Value
}

// SettingsPath is where our settings file lives. Override the directory with
// LABDASH_CONFIG_DIR.
func SettingsPath() string {
	return filepath.Join(ConfigDir(), settingsFile)
}

// LoadConfig reads our config. A missing file is not an error: it means "no
// instances configured yet", which is the normal state for a gitlab.com user.
func LoadConfig() (Config, error) {
	return LoadConfigFrom(SettingsPath())
}

// LoadConfigFrom reads a config from an explicit path.
func LoadConfigFrom(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{Instances: map[string]InstanceConfig{}}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", path, err)
	}

	cfg := Config{}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if cfg.Instances == nil {
		cfg.Instances = map[string]InstanceConfig{}
	}

	return cfg, nil
}

// SaveInstance writes one instance stanza into the settings file, creating the
// file if this is the first one.
//
// It writes the instance and nothing else. A credential never reaches this
// file: it goes to the keyring, or to a 0600 file beside it. The file stays
// safe to commit and safe to hand to a colleague, which is the whole reason
// customHeaders reads its secret from an environment variable.
//
// The file is created at this moment and not before. A starter file nobody
// asked for is the retired behaviour this replaces.
func SaveInstance(host string, inst InstanceConfig) error {
	return SaveInstanceTo(SettingsPath(), host, inst)
}

// SaveInstanceTo is SaveInstance against an explicit path.
func SaveInstanceTo(path, host string, inst InstanceConfig) error {
	if host == "" {
		return errors.New("an instance needs a host")
	}

	cfg, err := LoadConfigFrom(path)
	if err != nil {
		return err
	}
	if cfg.Instances == nil {
		cfg.Instances = map[string]InstanceConfig{}
	}
	cfg.Instances[host] = inst

	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	// Temp-and-rename, so an interrupted write cannot leave a settings file
	// that stops the application launching.
	tmp, err := os.CreateTemp(dir, settingsFile+".*")
	if err != nil {
		return fmt.Errorf("creating a temporary settings file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if _, err := tmp.Write(raw); err != nil {
		return fmt.Errorf("writing a temporary settings file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing a temporary settings file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}

	return nil
}

// Instance returns the settings for host, and whether an entry existed.
func (c Config) Instance(host string) (InstanceConfig, bool) {
	inst, ok := c.Instances[host]
	return inst, ok
}

// ResolveHost picks the host to use: an explicit choice, then the config's
// default, then $GITLAB_HOST and friends, then gitlab.com.
func (c Config) ResolveHost(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if c.DefaultHost != "" {
		return c.DefaultHost
	}
	return hostFromEnv()
}
