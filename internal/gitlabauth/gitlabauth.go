// Package gitlabauth owns labdash's credentials end to end: it logs in, it
// stores the result, it renews it, and it hands out ready-to-use credentials.
//
// It deliberately does NOT read glab's configuration or keyring. An earlier
// design borrowed glab's token as a zero-config convenience, and that turned
// out to be a trap: `glab auth login` mints an OAuth token that GitLab expires
// after two hours, and GitLab rotates the refresh token on use, so renewing a
// borrowed credential would have broken the user's glab. Owning the credential
// is what makes silent renewal possible at all.
//
// Nothing here ever prints, logs, or formats a token. Credentials and
// StoredToken both implement fmt.Stringer to redact themselves, so an
// accidental %v or %s cannot leak a secret.
package gitlabauth

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// DefaultHost is used when nothing else names one.
const DefaultHost = "gitlab.com"

// tokenEnvVars are the environment variables accepted for a token on the
// default host, in order of preference. They match glab's names so a user who
// already exports one does not have to learn a new variable.
//
// They apply to the DEFAULT host only. A dashboard talks to several instances
// at once, and a single global variable must never leak one instance's token
// to another; per-instance variables are named by `tokenEnv` in our config.
var tokenEnvVars = []string{"GITLAB_TOKEN", "GITLAB_ACCESS_TOKEN", "OAUTH_TOKEN"}

// hostEnvVars are the environment variables accepted for the default host.
var hostEnvVars = []string{"GITLAB_HOST", "GITLAB_URI", "GL_HOST"}

// Credentials is everything needed to build an authenticated HTTP client for
// one GitLab instance.
type Credentials struct {
	// Host is the instance the credentials belong to, e.g. "gitlab.com".
	Host string
	// APIHost is the hostname API calls go to. It differs from Host when the
	// instance sits behind a proxy or an SSH alias.
	APIHost string
	// APIProtocol is "https" or, rarely, "http".
	APIProtocol string
	// Subfolder is set when GitLab is installed under a path rather than at
	// the domain root, e.g. "gitlab" for https://example.com/gitlab/.
	Subfolder string

	// Token is the credential. Never print it.
	Token string
	// Source describes where Token came from, and is safe to display.
	Source string

	// Managed reports that the credential came from our own store, so Refresh
	// may renew it. A token supplied through an environment variable is not
	// managed: we did not mint it and cannot renew it.
	Managed bool
	// Kind is "oauth" or "pat".
	Kind TokenKind
	// IsOAuth2 is a convenience for Kind == KindOAuth.
	IsOAuth2 bool
	// OAuth2Expiry is when the access token dies. Zero for a PAT.
	OAuth2Expiry time.Time
	// RefreshToken, TokenType and ClientID are only populated for managed
	// OAuth credentials. Never print RefreshToken.
	RefreshToken string
	TokenType    string
	ClientID     string

	// Scopes is what the instance granted this credential, recorded at login so
	// that read-only is known before the first mutation rather than by failing
	// one. Empty means unknown. Only action.ScopeFromGranted reads it.
	Scopes []string

	// TLS and transport settings, from our instance config.
	CACert        string
	ClientCert    string
	ClientKey     string
	SkipTLSVerify bool
	Proxy         string
	CustomHeaders []Header
}

// String redacts both secrets so Credentials can be logged safely.
func (c Credentials) String() string {
	return fmt.Sprintf(
		"Credentials{Host:%s APIHost:%s Protocol:%s Subfolder:%q Token:<%d chars, redacted> "+
			"RefreshToken:<%d chars, redacted> Source:%s Kind:%s Managed:%t}",
		c.Host, c.APIHost, c.APIProtocol, c.Subfolder,
		len(c.Token), len(c.RefreshToken), c.Source, c.Kind, c.Managed)
}

// Describe is a one-line, token-free summary for the UI or a status command.
func (c Credentials) Describe() string {
	kind := "personal access token"
	if c.IsOAuth2 {
		kind = "OAuth token"
		if c.Managed {
			kind = "OAuth token (auto-renewing)"
		}
	}
	return fmt.Sprintf("%s: %d-character %s, hidden — from %s", c.Host, len(c.Token), kind, c.Source)
}

// GraphQLEndpoint returns the absolute URL of the instance's GraphQL endpoint.
func (c Credentials) GraphQLEndpoint() string {
	return c.baseURL() + "/api/graphql"
}

// RESTEndpoint returns the absolute URL of the instance's REST v4 root, with no
// trailing slash.
func (c Credentials) RESTEndpoint() string {
	return c.baseURL() + "/api/v4"
}

func (c Credentials) baseURL() string {
	proto := c.APIProtocol
	if proto == "" {
		proto = "https"
	}

	host := c.APIHost
	if host == "" {
		host = c.Host
	}

	base := proto + "://" + host
	if sub := strings.Trim(c.Subfolder, "/"); sub != "" {
		base += "/" + sub
	}

	return base
}

// OAuthExpired reports whether this is an OAuth credential whose expiry has
// passed.
//
// A zero expiry counts as expired. golang.org/x/oauth2 reads its own zero
// Expiry as "never expires", and a GitLab OAuth token that is never renewed
// dies silently after two hours — so the one reading that looks harmless is the
// one that breaks the dashboard with no error to explain it.
func (c Credentials) OAuthExpired() bool {
	if !c.IsOAuth2 {
		return false
	}
	if c.OAuth2Expiry.IsZero() {
		return true
	}
	return time.Now().After(c.OAuth2Expiry)
}

// NeedsRefresh reports whether a managed OAuth credential is close enough to
// expiry that it should be renewed before the next request. Renewing early
// means a slow fetch cannot straddle the boundary and 401 mid-render.
//
// A zero expiry needs a refresh, for the reason in OAuthExpired.
func (c Credentials) NeedsRefresh() bool {
	if !c.Managed || c.Kind != KindOAuth {
		return false
	}
	if c.OAuth2Expiry.IsZero() {
		return true
	}
	return time.Now().Add(refreshMargin).After(c.OAuth2Expiry)
}

// Advisory returns a warning worth showing the user once at startup, or "".
//
// With glab borrowing gone there is only one case left: an OAuth credential we
// own that has expired and carries no refresh token, which we cannot fix
// ourselves.
func (c Credentials) Advisory() string {
	if c.Managed && c.Kind == KindOAuth && c.OAuthExpired() && c.RefreshToken == "" {
		return fmt.Sprintf(
			"the stored OAuth token for %s expired and has no refresh token — "+
				"run `labdash auth login --hostname %s`", c.Host, c.Host)
	}
	return ""
}

// ErrNoToken means no credential was found for the host anywhere.
var ErrNoToken = errors.New("no GitLab credential found")

// NoTokenError carries the places that were searched, so the user gets an
// actionable message rather than a bare failure.
type NoTokenError struct {
	Host        string
	SearchedEnv []string
	StoreDesc   string
}

func (e *NoTokenError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "not logged in to %s\n\n", e.Host)
	fmt.Fprintf(&b, "  Log in:\n    labdash auth login --hostname %s\n\n", e.Host)
	fmt.Fprintf(&b, "  Or use a personal access token instead of OAuth:\n")
	fmt.Fprintf(&b, "    labdash auth login --hostname %s --with-token\n", e.Host)
	fmt.Fprintf(&b, "    (create one at https://%s/-/user_settings/personal_access_tokens"+
		"?scopes=api)\n\n", e.Host)
	if len(e.SearchedEnv) > 0 {
		fmt.Fprintf(&b, "  Searched environment: %s\n", strings.Join(e.SearchedEnv, ", "))
	}
	if e.StoreDesc != "" {
		fmt.Fprintf(&b, "  Searched credential store: %s\n", e.StoreDesc)
	}

	return b.String()
}

func (e *NoTokenError) Unwrap() error { return ErrNoToken }

// Options tunes credential resolution. The zero value is the normal case.
type Options struct {
	// Host to resolve. Empty means the config's defaultHost, then the
	// environment, then DefaultHost.
	Host string
	// SkipEnv ignores every environment variable, including an instance's
	// tokenEnv. For tests.
	SkipEnv bool
	// Config overrides our config. Nil means load it from disk.
	Config *Config
	// Store overrides our credential store. Nil means the default.
	Store *Store
}

// Resolve returns credentials for one GitLab instance.
//
// Order, most specific first:
//
//  1. the instance's own `tokenEnv` variable, if our config names one
//  2. GITLAB_TOKEN / GITLAB_ACCESS_TOKEN / OAUTH_TOKEN — default host only
//  3. our credential store, written by `labdash auth login`
//
// There is no fourth step. We never read glab's config or keyring.
//
// A credential from step 3 is Managed and can be renewed by Refresh. One from
// an environment variable is not: it is whatever the user exported.
//
// The token is never printed. Only Source and Describe are safe to display.
func Resolve(opts Options) (Credentials, error) {
	cfg := Config{}
	if opts.Config != nil {
		cfg = *opts.Config
	} else {
		loaded, err := LoadConfig()
		if err != nil {
			return Credentials{}, err
		}
		cfg = loaded
	}

	host := cfg.ResolveHost(opts.Host)
	inst, _ := cfg.Instance(host)

	creds := Credentials{
		Host:          host,
		APIHost:       host,
		APIProtocol:   "https",
		CustomHeaders: inst.CustomHeaders,
	}
	applyInstance(&creds, inst)

	if !opts.SkipEnv {
		// An instance-specific variable is the most explicit signal there is,
		// so it wins even over our own stored credential.
		if inst.TokenEnv != "" {
			if v := os.Getenv(inst.TokenEnv); v != "" {
				creds.Token = v
				creds.Kind = KindPAT
				creds.Source = "$" + inst.TokenEnv
				return creds, nil
			}
		}

		// The generic variables are not host-scoped, so they only apply to the
		// default host. Otherwise `--hostname other.example.com` would silently
		// send gitlab.com's token to another instance.
		if opts.Host == "" || opts.Host == cfg.ResolveHost("") {
			if tok, name := tokenFromEnv(); tok != "" {
				creds.Token = tok
				creds.Kind = KindPAT
				creds.Source = "$" + name
				return creds, nil
			}
		}
	}

	store := opts.Store
	if store == nil {
		store = NewStore()
	}

	if stored, err := store.Load(host); err == nil {
		managed := credentialsFromStored(host, stored, store.Location(host))
		// Instance settings describe the instance, not the credential.
		managed.APIHost = creds.APIHost
		managed.APIProtocol = creds.APIProtocol
		managed.Subfolder = creds.Subfolder
		managed.CACert = creds.CACert
		managed.ClientCert = creds.ClientCert
		managed.ClientKey = creds.ClientKey
		managed.SkipTLSVerify = creds.SkipTLSVerify
		managed.Proxy = creds.Proxy
		managed.CustomHeaders = creds.CustomHeaders
		return managed, nil
	}

	searched := tokenEnvVars
	if inst.TokenEnv != "" {
		searched = append([]string{inst.TokenEnv}, searched...)
	}

	return creds, &NoTokenError{
		Host:        host,
		SearchedEnv: searched,
		StoreDesc:   store.Describe(host),
	}
}

func applyInstance(creds *Credentials, inst InstanceConfig) {
	if inst.APIHost != "" {
		creds.APIHost = inst.APIHost
	}
	if inst.APIProtocol != "" {
		creds.APIProtocol = inst.APIProtocol
	}

	creds.Subfolder = inst.Subfolder
	creds.CACert = inst.CACert
	creds.ClientCert = inst.ClientCert
	creds.ClientKey = inst.ClientKey
	creds.SkipTLSVerify = inst.InsecureSkipVerify
	creds.Proxy = inst.Proxy
}

func hostFromEnv() string {
	for _, name := range hostEnvVars {
		if v := os.Getenv(name); v != "" {
			return strings.TrimSuffix(strings.TrimPrefix(v, "https://"), "/")
		}
	}
	return DefaultHost
}

func tokenFromEnv() (value, name string) {
	for _, n := range tokenEnvVars {
		if v := os.Getenv(n); v != "" {
			return v, n
		}
	}
	return "", ""
}
