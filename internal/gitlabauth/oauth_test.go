package gitlabauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newTestStore returns a Store backed by a temp dir with the keyring disabled,
// so tests never touch the developer's real credential manager.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	return &Store{Dir: t.TempDir(), DisableKeyring: true}
}

func TestStoreRoundTrip(t *testing.T) {
	s := newTestStore(t)
	want := StoredToken{
		AccessToken:  "access-value",
		RefreshToken: "refresh-value",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(2 * time.Hour).Truncate(time.Second),
		ClientID:     "client-abc",
		Kind:         KindOAuth,
	}

	if err := s.Save("gitlab.com", want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Load("gitlab.com")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Errorf("round trip lost a token value")
	}
	if !got.Expiry.Equal(want.Expiry) {
		t.Errorf("Expiry = %s, want %s", got.Expiry, want.Expiry)
	}
	if got.ClientID != want.ClientID || got.Kind != want.Kind {
		t.Errorf("round trip lost metadata: %+v", got)
	}

	removed, err := s.Delete("gitlab.com")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !removed {
		t.Error("Delete reported nothing was removed, but a credential was stored")
	}
	if _, err := s.Load("gitlab.com"); !errors.Is(err, ErrNoStoredToken) {
		t.Errorf("after Delete, Load error = %v, want ErrNoStoredToken", err)
	}

	// Deleting again must report honestly that there was nothing to remove.
	removed, err = s.Delete("gitlab.com")
	if err != nil {
		t.Fatalf("second Delete: %v", err)
	}
	if removed {
		t.Error("Delete claimed to remove a credential that was already gone")
	}
}

func TestStoreKeepsHostsSeparate(t *testing.T) {
	s := newTestStore(t)

	for _, h := range []string{"gitlab.com", "gitlab.example.com"} {
		if err := s.Save(h, StoredToken{AccessToken: "token-for-" + h, Kind: KindOAuth}); err != nil {
			t.Fatalf("Save(%s): %v", h, err)
		}
	}

	for _, h := range []string{"gitlab.com", "gitlab.example.com"} {
		got, err := s.Load(h)
		if err != nil {
			t.Fatalf("Load(%s): %v", h, err)
		}
		if got.AccessToken != "token-for-"+h {
			t.Errorf("Load(%s) returned %q — hosts are bleeding into each other", h, got.AccessToken)
		}
	}

	// Deleting one host must leave the other intact.
	if _, err := s.Delete("gitlab.com"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Load("gitlab.example.com"); err != nil {
		t.Errorf("deleting one host removed another: %v", err)
	}
}

func TestStoredTokenRedactsBothSecrets(t *testing.T) {
	tok := StoredToken{AccessToken: "SECRET-ACCESS", RefreshToken: "SECRET-REFRESH"}

	s := tok.String()
	for _, secret := range []string{"SECRET-ACCESS", "SECRET-REFRESH"} {
		if strings.Contains(s, secret) {
			t.Errorf("%s leaked through String(): %s", secret, s)
		}
	}
}

// AUT-04.T1 — the five-minute margin.
//
// Prevents: refreshing exactly at expiry and losing the race, so that a slow
// fetch straddles the boundary and 401s halfway through a render.
func TestAUT04T1_TheFiveMinuteRefreshMarginHolds(t *testing.T) {
	tests := []struct {
		name string
		c    Credentials
		want bool
	}{
		{
			name: "managed oauth well inside its life",
			c:    Credentials{Managed: true, Kind: KindOAuth, OAuth2Expiry: time.Now().Add(time.Hour)},
			want: false,
		},
		{
			name: "managed oauth expiring in four minutes",
			c:    Credentials{Managed: true, Kind: KindOAuth, OAuth2Expiry: time.Now().Add(4 * time.Minute)},
			want: true,
		},
		{
			name: "managed oauth expiring in six minutes",
			c:    Credentials{Managed: true, Kind: KindOAuth, OAuth2Expiry: time.Now().Add(6 * time.Minute)},
			want: false,
		},
		{
			name: "managed oauth already expired",
			c:    Credentials{Managed: true, Kind: KindOAuth, OAuth2Expiry: time.Now().Add(-time.Hour)},
			want: true,
		},
		{
			name: "managed PAT never needs refreshing",
			c:    Credentials{Managed: true, Kind: KindPAT, OAuth2Expiry: time.Now().Add(-time.Hour)},
			want: false,
		},
		{
			name: "an unmanaged token is never ours to refresh",
			c:    Credentials{Managed: false, IsOAuth2: true, OAuth2Expiry: time.Now().Add(-time.Hour)},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.NeedsRefresh(); got != tt.want {
				t.Errorf("NeedsRefresh() = %t, want %t", got, tt.want)
			}
		})
	}
}

// AUT-04.T3 and REG-07 — a zero expiry is expired, never "never expires".
//
// Prevents: the golang.org/x/oauth2 zero-value trap. The library reads its own
// zero Expiry as a token that never lapses, so a credential stored without one
// is never refreshed and dies silently after two hours, with no error to
// explain why the dashboard stopped.
func TestAUT04T3_REG07_AZeroExpiryMeansExpired(t *testing.T) {
	oauthCred := Credentials{Managed: true, Kind: KindOAuth, IsOAuth2: true}

	if !oauthCred.OAuthExpired() {
		t.Error("OAuthExpired() = false for a zero expiry; that is the oauth2 trap")
	}
	if !oauthCred.NeedsRefresh() {
		t.Error("NeedsRefresh() = false for a zero expiry, so the token would never be renewed")
	}

	// A personal access token legitimately has no expiry we know of, and
	// nothing can refresh one anyway.
	pat := Credentials{Managed: true, Kind: KindPAT}
	if pat.OAuthExpired() || pat.NeedsRefresh() {
		t.Error("a personal access token was treated as an expired OAuth credential")
	}
}

// AUT-04.T4 — Refresh must be a no-op for a credential we did not mint. We hold
// no refresh token for it, so there is nothing to exchange.
//
// Prevents: attempting to refresh a credential we did not mint.
func TestAUT04T4_RefreshIgnoresACredentialWeDidNotMint(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	unmanaged := Credentials{
		Host: "gitlab.com", Token: "from-env", RefreshToken: "not-ours",
		IsOAuth2: true, Managed: false, OAuth2Expiry: time.Now().Add(-time.Hour),
	}

	got, err := Refresh(context.Background(), unmanaged, LoginOptions{HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if called {
		t.Error("Refresh made a token request for a credential it does not own")
	}
	if got.Token != "from-env" {
		t.Error("Refresh altered a credential it does not own")
	}
}

// AUT-04.T2 — a managed credential inside the margin is renewed AND persisted,
// since GitLab invalidates the old refresh token the moment the new one is
// issued.
//
// Prevents: the next refresh failing forever because GitLab already
// invalidated the token we still hold.
func TestAUT04T2_RefreshPersistsTheRotatedToken(t *testing.T) {
	var gotGrant, gotRefresh string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		form, _ := url.ParseQuery(string(body))
		gotGrant = form.Get("grant_type")
		gotRefresh = form.Get("refresh_token")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "access_token":"new-access",
            "refresh_token":"new-refresh",
            "token_type":"Bearer",
            "expires_in":7200
        }`))
	}))
	defer srv.Close()

	store := newTestStore(t)
	host := strings.TrimPrefix(srv.URL, "http://")

	creds := Credentials{
		Host: host, APIHost: host, APIProtocol: "http",
		Token: "old-access", RefreshToken: "old-refresh", TokenType: "Bearer",
		ClientID: "client-abc", Kind: KindOAuth, IsOAuth2: true, Managed: true,
		OAuth2Expiry: time.Now().Add(time.Minute),
	}

	got, err := Refresh(context.Background(), creds, LoginOptions{
		Store:      store,
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if gotGrant != "refresh_token" {
		t.Errorf("grant_type = %q, want refresh_token", gotGrant)
	}
	if gotRefresh != "old-refresh" {
		t.Errorf("sent refresh_token = %q, want old-refresh", gotRefresh)
	}
	if got.Token != "new-access" {
		t.Errorf("Token = %q, want new-access", got.Token)
	}

	// The rotated refresh token must be on disk before we return; GitLab has
	// already invalidated the old one.
	stored, err := store.Load(host)
	if err != nil {
		t.Fatalf("Load after refresh: %v", err)
	}
	if stored.RefreshToken != "new-refresh" {
		t.Errorf("persisted refresh token = %q, want new-refresh — the next refresh would fail",
			stored.RefreshToken)
	}

	// Host settings must survive the renewal.
	if got.APIProtocol != "http" {
		t.Errorf("APIProtocol = %q, want the original http", got.APIProtocol)
	}
}

func TestLoginWithoutClientIDIsActionable(t *testing.T) {
	t.Setenv(ClientIDEnvVar, "")

	_, err := Login(context.Background(), LoginOptions{
		Host:  "gitlab.example.com",
		Store: newTestStore(t),
	})
	if err == nil {
		t.Fatal("Login succeeded with no client ID")
	}
	if !errors.Is(err, ErrNoClientID) {
		t.Errorf("error does not unwrap to ErrNoClientID: %v", err)
	}

	msg := err.Error()
	for _, want := range []string{
		"user_settings/applications",
		"http://localhost:7171/auth/redirect",
		ClientIDEnvVar,
		"personal access token",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message is missing %q:\n%s", want, msg)
		}
	}
}

func TestLoginUsesClientIDFromEnv(t *testing.T) {
	t.Setenv(ClientIDEnvVar, "client-from-env")

	got, err := resolveClientID("gitlab.example.com", "", nil)
	if err != nil {
		t.Fatalf("resolveClientID: %v", err)
	}
	if got != "client-from-env" {
		t.Errorf("clientID = %q, want client-from-env", got)
	}

	// An explicit value beats the environment.
	got, err = resolveClientID("gitlab.example.com", "explicit", nil)
	if err != nil {
		t.Fatalf("resolveClientID: %v", err)
	}
	if got != "explicit" {
		t.Errorf("clientID = %q, want explicit", got)
	}
}

func TestBaseURLForSelectsBuiltInEndpointsForGitLabCom(t *testing.T) {
	if got := oauthBaseURL("https", "gitlab.com", ""); got != "" {
		t.Errorf("oauthBaseURL(gitlab.com) = %q, want \"\" so gitlaboauth2 uses its built-ins", got)
	}
	if got := oauthBaseURL("https", "gitlab.example.com", ""); got != "https://gitlab.example.com" {
		t.Errorf("oauthBaseURL(self-managed) = %q", got)
	}
}

func TestIsPortInUse(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "unix",
			err:  errors.New("listen tcp 127.0.0.1:7171: bind: address already in use"),
			want: true,
		},
		{
			name: "windows",
			err: errors.New("listen tcp 127.0.0.1:7171: bind: Only one usage of each " +
				"socket address (protocol/network address/port) is normally permitted."),
			want: true,
		},
		{name: "a real auth failure is not a port clash", err: errors.New("invalid_grant"), want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPortInUse(tt.err); got != tt.want {
				t.Errorf("isPortInUse(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}

// Guard against the isolation bug that leaked a real credential into test
// output on 2026-08-01: a Resolve without Store or SkipManaged reads the
// developer's own keyring. Every test in this package must opt out.
func TestResolveIsIsolatedFromTheRealStore(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITLAB_ACCESS_TOKEN", "")
	t.Setenv("OAUTH_TOKEN", "")

	got, err := Resolve(Options{
		Host:   "gitlab.com",
		Config: &Config{},
		Store:  newTestStore(t),
	})
	if err == nil {
		t.Fatalf("an empty test store must yield no credential, got %d chars from %s",
			len(got.Token), got.Source)
	}
	if !errors.Is(err, ErrNoToken) {
		t.Errorf("error does not unwrap to ErrNoToken: %v", err)
	}
}
