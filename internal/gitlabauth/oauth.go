package gitlabauth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gitlab.com/gitlab-org/api/client-go/v2/gitlaboauth2"
	"golang.org/x/oauth2"
)

// DefaultClientID is the OAuth application ID labdash uses on gitlab.com.
//
// This is NOT a secret and is not treated as one. For a public (non-confidential)
// PKCE client the client ID is published by design: it travels in the URL of every
// authorization request, so the user sees it in their own address bar. RFC 6749
// §2.2 says so outright — "the client identifier is not a secret". glab ships
// its own in public source (gitlab-org/cli, internal/glinstance/host.go), and
// so does the GitHub CLI.
//
// What actually protects a public client is PKCE plus the redirect-URI
// allowlist, both of which we use. Someone holding this ID cannot obtain a
// token: the authorization code only ever lands on the victim's own
// localhost:7171, and without the PKCE verifier it cannot be exchanged.
//
// It is a var rather than a const so a fork can substitute its own application
// at build time without editing source:
//
//	go build -ldflags "-X github.com/giancarlosisasi/labdash/internal/gitlabauth.DefaultClientID=<id>"
//
// That is a packaging convenience, not a security measure — `strings` finds an
// ldflags value in a binary just as easily as a source constant.
//
// GitLab's UI labels this "Application ID"; the OAuth spec calls it client_id.
// They are the same value.
//
// Registered on gitlab.com as "labdash", owned by the public group
// gitlab.com/labdash rather than by an individual account. Group ownership is
// what lets users other than the maintainer authorize it, and it means the
// application outlives any one account.
//
// Settings: confidential off, device authorization grant on, scopes
// api + read_api + read_user, callback http://localhost:7171/auth/redirect.
//
// Self-managed instances need their own application — see
// research/09-auth-strategy.md section 2.1 — supplied with --client-id or
// LABDASH_CLIENT_ID.
var DefaultClientID = "e26892e5b17efbc8e88d4994da66a7c724fc371782caf5440fdba000850b4f0f"

// ClientIDEnvVar lets a user supply a client ID without editing any config,
// which is how self-managed instances will most often be set up.
const ClientIDEnvVar = "LABDASH_CLIENT_ID"

// Scopes.
//
// read_user is what lets us greet the user by name on startup: GitLab documents
// it as "username, public email, and full name". It is strictly read-only
// profile data.
//
// We still ask for materially less than glab, which requests
// "openid profile read_user write_repository api" — we never write to a
// repository, and we do not need OpenID Connect.
var (
	writeScopes = []string{"api", "read_user"}
	readScopes  = []string{"read_api", "read_user"}
)

// refreshMargin is how long before real expiry we renew. GitLab OAuth access
// tokens live two hours; renewing five minutes early means a slow fetch can
// never straddle the boundary and 401 halfway through a render.
const refreshMargin = 5 * time.Minute

// LoginMethod selects the OAuth flow.
type LoginMethod string

const (
	// MethodAuto tries the device flow and falls back to the browser flow.
	MethodAuto LoginMethod = "auto"
	// MethodDevice is the Device Authorization Grant, RFC 8628. It needs no
	// callback listener, so it works over SSH, in WSL, and in containers.
	// Requires GitLab 17.9 or later.
	MethodDevice LoginMethod = "device"
	// MethodBrowser is the authorization-code flow with PKCE over a loopback
	// listener. Needed for GitLab older than 17.9.
	MethodBrowser LoginMethod = "browser"
)

// LoginOptions configures an interactive login.
type LoginOptions struct {
	Host     string
	ClientID string
	ReadOnly bool
	Method   LoginMethod

	// Out receives the user-facing prompts. Defaults to io.Discard.
	Out io.Writer
	// Browser opens a URL. Defaults to the platform opener.
	Browser func(url string) error
	// NoBrowser stops the device flow opening a browser, for SSH sessions and
	// headless machines where the URL goes to another device by hand.
	NoBrowser bool
	// HTTPClient is used for the token exchange. Defaults to http.DefaultClient.
	HTTPClient *http.Client
	// Store persists the result. Defaults to NewStore().
	Store *Store
	// Instance supplies this host's settings (apiHost, subfolder, TLS). Nil
	// means the defaults, which is correct for gitlab.com.
	Instance *InstanceConfig
}

// ErrNoClientID means we have no OAuth application to authenticate against.
var ErrNoClientID = errors.New("no OAuth client ID configured")

// Login runs an interactive OAuth flow and stores the resulting credential in
// our own store.
func Login(ctx context.Context, opts LoginOptions) (Credentials, error) {
	host := opts.Host
	if host == "" {
		host = hostFromEnv()
	}

	clientID, err := resolveClientID(host, opts.ClientID, opts.Instance)
	if err != nil {
		return Credentials{}, err
	}

	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	store := opts.Store
	if store == nil {
		store = NewStore()
	}
	method := opts.Method
	if method == "" {
		method = MethodAuto
	}

	scopes := writeScopes
	if opts.ReadOnly {
		scopes = readScopes
	}

	ctx = withHTTPClient(ctx, opts.HTTPClient)

	base := Credentials{Host: host, APIHost: host, APIProtocol: "https"}
	if opts.Instance != nil {
		applyInstance(&base, *opts.Instance)
	}
	cfg := gitlaboauth2.NewOAuth2Config(base.oauthBaseURL(), clientID, "", scopes)

	// The device flow opens the browser as a convenience; NoBrowser turns that
	// off for SSH sessions and headless machines, where the URL is meant to be
	// carried to another device by hand.
	opener := opts.Browser
	if opener == nil {
		opener = openBrowser
	}
	if opts.NoBrowser {
		opener = nil
	}

	var tok *oauth2.Token
	switch method {
	case MethodBrowser:
		tok, err = browserFlow(ctx, host, clientID, scopes, opts.Browser)
	case MethodDevice:
		tok, err = deviceFlow(ctx, cfg, out, opener)
	default:
		tok, err = deviceFlow(ctx, cfg, out, opener)
		if err != nil && isDeviceFlowUnsupported(err) {
			fmt.Fprintf(out, "\n  This instance does not support the device flow "+
				"(it needs GitLab 17.9 or later). Falling back to the browser.\n")
			tok, err = browserFlow(ctx, host, clientID, scopes, opts.Browser)
		}
	}
	if err != nil {
		return Credentials{}, err
	}

	stored := StoredToken{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		Expiry:       tok.Expiry,
		ClientID:     clientID,
		Kind:         KindOAuth,
	}
	if err := store.Save(host, stored); err != nil {
		return Credentials{}, fmt.Errorf("saving credential: %w", err)
	}

	logged := credentialsFromStored(host, stored, store.Location(host))
	logged.APIHost = base.APIHost
	logged.APIProtocol = base.APIProtocol
	logged.Subfolder = base.Subfolder

	return logged, nil
}

// LoginWithToken validates a personal access token and stores it, instead of
// running an OAuth flow.
//
// This is the path for self-managed instances, which cannot use the gitlab.com
// OAuth application and would otherwise need an admin to register one. It is
// also the escape hatch for anyone who simply prefers a PAT.
//
// The token is validated against the instance before it is stored, so a typo
// fails here rather than on the first dashboard refresh.
func LoginWithToken(ctx context.Context, token string, opts LoginOptions) (Credentials, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Credentials{}, errors.New("no token was supplied")
	}

	host := opts.Host
	if host == "" {
		host = hostFromEnv()
	}
	store := opts.Store
	if store == nil {
		store = NewStore()
	}

	probe := Credentials{
		Host: host, APIHost: host, APIProtocol: "https", Token: token, Kind: KindPAT,
	}
	if opts.Instance != nil {
		applyInstance(&probe, *opts.Instance)
	}

	if _, err := WhoAmI(ctx, probe, opts.HTTPClient); err != nil {
		return Credentials{}, fmt.Errorf("that token did not work: %w", err)
	}

	stored := StoredToken{AccessToken: token, Kind: KindPAT}
	if err := store.Save(host, stored); err != nil {
		return Credentials{}, fmt.Errorf("saving credential: %w", err)
	}

	out := credentialsFromStored(host, stored, store.Location(host))
	out.APIHost = probe.APIHost
	out.APIProtocol = probe.APIProtocol
	out.Subfolder = probe.Subfolder

	return out, nil
}

// Logout removes our stored credential for host and reports whether there was
// one.
//
// Note what this does NOT do: the token stays valid at GitLab until it expires
// or the user revokes the application authorization. Forgetting a key is not
// the same as revoking it, and the caller should say so.
func Logout(host string, store *Store) (removed bool, err error) {
	if store == nil {
		store = NewStore()
	}
	if host == "" {
		host = hostFromEnv()
	}

	return store.Delete(host)
}

// RevokeURL is where a user goes to actually invalidate the token at GitLab,
// rather than merely forgetting it locally.
func RevokeURL(host string) string {
	if host == "" {
		host = DefaultHost
	}
	return "https://" + host + "/-/user_settings/applications"
}

// Refresh returns credentials guaranteed usable for at least refreshMargin.
//
// It renews only credentials from our own store. A token supplied through an
// environment variable is returned untouched: we did not mint it, so we hold
// no refresh token for it.
func Refresh(ctx context.Context, creds Credentials, opts LoginOptions) (Credentials, error) {
	if !creds.Managed || creds.Kind != KindOAuth {
		return creds, nil
	}
	if !creds.NeedsRefresh() {
		return creds, nil
	}
	if creds.RefreshToken == "" {
		return creds, fmt.Errorf(
			"the stored OAuth token for %s expired and carries no refresh token — "+
				"run `labdash auth login --hostname %s`", creds.Host, creds.Host)
	}

	store := opts.Store
	if store == nil {
		store = NewStore()
	}

	clientID := creds.ClientID
	if clientID == "" {
		var err error
		if clientID, err = resolveClientID(creds.Host, opts.ClientID, opts.Instance); err != nil {
			return creds, err
		}
	}

	ctx = withHTTPClient(ctx, opts.HTTPClient)
	cfg := gitlaboauth2.NewOAuth2Config(creds.oauthBaseURL(), clientID, "", nil)

	// TokenSource renews only when the token is invalid. A ZERO Expiry counts
	// as "never expires" in golang.org/x/oauth2, so it must be stamped in the
	// past to force the exchange — we have already decided a refresh is due,
	// and oauth2's own 10-second margin is far tighter than our 5-minute one.
	current := &oauth2.Token{
		AccessToken:  creds.Token,
		RefreshToken: creds.RefreshToken,
		TokenType:    creds.TokenType,
		Expiry:       time.Now().Add(-time.Second),
	}

	refreshed, err := cfg.TokenSource(ctx, current).Token()
	if err != nil {
		return creds, fmt.Errorf(
			"refreshing the OAuth token for %s failed (%w) — "+
				"run `labdash auth login --hostname %s`", creds.Host, err, creds.Host)
	}

	stored := StoredToken{
		AccessToken:  refreshed.AccessToken,
		RefreshToken: refreshed.RefreshToken,
		TokenType:    refreshed.TokenType,
		Expiry:       refreshed.Expiry,
		ClientID:     clientID,
		Kind:         KindOAuth,
	}
	// GitLab rotates the refresh token on every use, so persisting the new one
	// immediately is not an optimisation — miss it and the next refresh fails.
	if err := store.Save(creds.Host, stored); err != nil {
		return creds, fmt.Errorf("saving refreshed credential: %w", err)
	}

	out := credentialsFromStored(creds.Host, stored, store.Location(creds.Host))
	out.APIHost = creds.APIHost
	out.APIProtocol = creds.APIProtocol
	out.Subfolder = creds.Subfolder

	return out, nil
}

// ---------------------------------------------------------------------------
// Flows
// ---------------------------------------------------------------------------

func deviceFlow(
	ctx context.Context,
	cfg *oauth2.Config,
	out io.Writer,
	browser func(string) error,
) (*oauth2.Token, error) {
	auth, err := cfg.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting device authorization: %w", err)
	}

	// VerificationURIComplete carries the code in the query string, so opening
	// it fills the form in for the user. Every manual step removed here is a
	// step that cannot be mistyped or done too late — the code only lives for
	// five minutes.
	link := auth.VerificationURIComplete
	if link == "" {
		link = auth.VerificationURI
	}

	fmt.Fprintf(out, "\n  One-time code:  %s\n", auth.UserCode)
	fmt.Fprintf(out, "  Approve it at:  %s\n", link)

	if !auth.Expiry.IsZero() {
		fmt.Fprintf(out, "\n  This code expires at %s, %s from now.\n",
			auth.Expiry.Format("15:04:05"), time.Until(auth.Expiry).Round(time.Second))
	}

	if browser != nil {
		if err := browser(link); err == nil {
			fmt.Fprintf(out, "  Opening your browser now — approve there and this finishes itself.\n")
		} else {
			fmt.Fprintf(out, "  (could not open a browser automatically: %v)\n", err)
		}
	}

	fmt.Fprintf(out, "\n  Waiting for authorization...\n")

	tok, err := cfg.DeviceAccessToken(ctx, auth)
	if err != nil {
		return nil, describeDeviceError(err, auth)
	}

	return tok, nil
}

// describeDeviceError turns the two failures a user actually causes into plain
// sentences. Everything else keeps its original wording.
func describeDeviceError(err error, auth *oauth2.DeviceAuthResponse) error {
	expired := fmt.Errorf(
		"the one-time code %s expired before it was approved.\n"+
			"  GitLab only keeps a device code for five minutes.\n"+
			"  Run `labdash auth login` again — it opens the browser for you, so the\n"+
			"  only step left is clicking Authorize.\n"+
			"  Codes from an earlier or interrupted run are already dead; always use the\n"+
			"  one printed by the run that is still waiting", auth.UserCode)

	if errors.Is(err, context.DeadlineExceeded) {
		return expired
	}

	var re *oauth2.RetrieveError
	if errors.As(err, &re) {
		switch re.ErrorCode {
		case "expired_token":
			return expired
		case "access_denied":
			return fmt.Errorf(
				"authorization was declined in the browser — nothing was stored")
		}
	}

	return fmt.Errorf("device authorization: %w", err)
}

func browserFlow(
	ctx context.Context,
	host, clientID string,
	scopes []string,
	browser func(string) error,
) (*oauth2.Token, error) {
	if browser == nil {
		browser = openBrowser
	}

	// Why a fixed port rather than an ephemeral one:
	//
	// GitLab compares redirect_uri against the application's registered value
	// as an exact string, so the port cannot vary. RFC 8252 section 7.3 asks
	// authorization servers to ignore the port for loopback redirects, but that
	// only applies when the host is the IP literal 127.0.0.1 or ::1 — our
	// registered URI uses the hostname "localhost", so the exemption would not
	// apply even on a server that implements it.
	//
	// 7171 is the port glab uses, so a self-managed admin registering either
	// tool writes the same callback URL.
	//
	// We bind localhost explicitly where glab binds ":7171" (every interface).
	// Ours is the stricter choice: nothing else on the network can reach the
	// callback listener, which exists for only a few seconds anyway.
	const (
		redirectURL = "http://localhost:7171/auth/redirect"
		listenAddr  = "localhost:7171"
	)

	tok, err := gitlaboauth2.AuthorizationFlow(
		ctx,
		baseURLFor(host),
		clientID,
		redirectURL,
		scopes,
		listenAddr,
		browser,
	)
	if err != nil {
		if isPortInUse(err) {
			return nil, fmt.Errorf(
				"port 7171 is already in use, so the browser flow cannot receive its callback.\n"+
					"  The likeliest cause is a `glab auth login` still waiting — glab uses the\n"+
					"  same port. Finish or cancel it, then retry.\n"+
					"  Or drop --web: the device flow needs no listener at all.\n\n"+
					"  underlying error: %w", err)
		}
		return nil, fmt.Errorf("browser authorization: %w", err)
	}

	return tok, nil
}

// isPortInUse reports whether a flow failed because the callback port was
// taken, which deserves a different message from a real authorization failure.
// Matching on text rather than syscall.EADDRINUSE keeps this working across
// platforms and through the wrapping the oauth2 helper does.
func isPortInUse(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	for _, s := range []string{
		"address already in use",
		"only one usage of each socket address",
		"bind: permission denied",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}

	return false
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func oauthConfig(host, clientID string, scopes []string, redirectURL string) *oauth2.Config {
	return gitlaboauth2.NewOAuth2Config(baseURLFor(host), clientID, redirectURL, scopes)
}

// baseURLFor returns "" for gitlab.com, which makes gitlaboauth2 use its
// built-in endpoints, and an https URL for anything else.
func baseURLFor(host string) string {
	return oauthBaseURL("https", host, "")
}

// oauthBaseURL builds the instance root that /oauth/authorize_device and
// /oauth/token hang off. Returning "" for plain https gitlab.com hands the job
// to gitlaboauth2's built-in endpoints.
//
// The OAuth endpoints live on the web host, not on api_host: only the API is
// ever proxied elsewhere.
func oauthBaseURL(protocol, host, subfolder string) string {
	if protocol == "" {
		protocol = "https"
	}
	if host == "" || (host == DefaultHost && protocol == "https" && subfolder == "") {
		return ""
	}

	base := protocol + "://" + host
	if sub := strings.Trim(subfolder, "/"); sub != "" {
		base += "/" + sub
	}

	return base
}

// oauthBaseURL is the instance root for this credential's own instance,
// honouring its protocol and subfolder.
func (c Credentials) oauthBaseURL() string {
	return oauthBaseURL(c.APIProtocol, c.Host, c.Subfolder)
}

func withHTTPClient(ctx context.Context, c *http.Client) context.Context {
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, oauth2.HTTPClient, c)
}

// resolveClientID finds the OAuth application to authenticate against:
// explicit flag, then LABDASH_CLIENT_ID, then the built-in gitlab.com
// application. Self-managed instances have no built-in, exactly as with glab.
func resolveClientID(host, explicit string, inst *InstanceConfig) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if v := os.Getenv(ClientIDEnvVar); v != "" {
		return v, nil
	}
	if inst != nil && inst.ClientID != "" {
		return inst.ClientID, nil
	}
	if host == DefaultHost && DefaultClientID != "" {
		return DefaultClientID, nil
	}

	return "", fmt.Errorf(
		"%w for %s\n\n"+
			"  OAuth needs an application registered on the instance.\n"+
			"  Create one at  https://%s/-/user_settings/applications  with:\n"+
			"    Redirect URI : http://localhost:7171/auth/redirect\n"+
			"    Confidential : unchecked\n"+
			"    Scopes       : api  (and read_api for --read-only)\n\n"+
			"    Device authorization grant: CHECKED\n\n"+
			"  Then pass the Application ID:\n"+
			"    labdash auth login --hostname %s --client-id <id>\n"+
			"  or set %s, or add `clientId:` for this instance in\n"+
			"    %s\n\n"+
			"  Or skip OAuth entirely and use a personal access token, which needs no\n"+
			"  application at all:\n"+
			"    labdash auth login --hostname %s --with-token\n"+
			"    (create one at https://%s/-/user_settings/personal_access_tokens)",
		ErrNoClientID, host, host, host, ClientIDEnvVar, SettingsPath(), host, host)
}

// isDeviceFlowUnsupported reports whether the failure means "this instance is
// too old for RFC 8628" rather than a real error. GitLab returns an
// unsupported grant type, and pre-17.1 instances 404 the endpoint.
func isDeviceFlowUnsupported(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	for _, s := range []string{
		"unsupported_grant_type",
		"unsupported grant",
		"404",
		"not found",
		"invalid_request",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}

	return false
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func credentialsFromStored(host string, tok StoredToken, source string) Credentials {
	return Credentials{
		Host:         host,
		APIHost:      host,
		APIProtocol:  "https",
		Token:        tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		OAuth2Expiry: tok.Expiry,
		ClientID:     tok.ClientID,
		Kind:         tok.Kind,
		IsOAuth2:     tok.Kind == KindOAuth,
		Managed:      true,
		Source:       source,
	}
}
