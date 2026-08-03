package gitlabauth

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/fakegitlab"
)

// fakeHost turns a running fake into the host and instance settings a login
// needs. The fake speaks plain http, so the instance carries apiProtocol.
func fakeHost(srv *fakegitlab.Server) (string, InstanceConfig) {
	return strings.TrimPrefix(srv.URL, "http://"), InstanceConfig{APIProtocol: "http"}
}

// AUT-01.T1 — the device flow end to end.
//
// Prevents: a login that stores nothing, stores something unmanaged, or polls
// faster than the instance allowed. GitLab answers an impatient client with
// slow_down, and a client that ignores the advertised interval earns it.
func TestAUT01T1_DeviceFlowStoresAManagedCredential(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t, fakegitlab.WithDevicePolls(1))
	host, inst := fakeHost(srv)
	store := newTestStore(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	creds, err := Login(ctx, LoginOptions{
		Host: host, ClientID: "test-client", Method: MethodDevice,
		Instance: &inst, Store: store, HTTPClient: srv.Client(), NoBrowser: true,
		Out: io.Discard,
	})
	require.NoError(t, err)

	require.True(t, creds.Managed, "a credential from our own flow must be renewable")
	require.Equal(t, KindOAuth, creds.Kind)
	require.NotEmpty(t, creds.RefreshToken, "without a refresh token nothing can renew")

	stored, err := store.Load(host)
	require.NoError(t, err)
	require.Equal(t, creds.Token, stored.AccessToken, "the credential was not persisted")

	// Our use of RFC 8628 is that the instance's own device-authorization
	// response is handed to the poller untouched, so the interval it asked for
	// is the interval used. A flow that expected a token on the first request
	// would show one call here.
	require.Equal(t, 2, srv.Calls("oauth.token"),
		"the flow should have polled once, been told to wait, and polled again")
}

// AUT-01.T2 — an instance too old for RFC 8628.
//
// Prevents: every pre-17.9 self-managed user being unable to log in. The
// fallback must happen without asking the user to know what a device grant is.
func TestAUT01T2_DeviceFlowFallsBackToTheBrowser(t *testing.T) {
	t.Run("the rejection is recognised", func(t *testing.T) {
		t.Parallel()

		for _, msg := range []string{
			"unsupported_grant_type",
			"oauth2: cannot auth device: 404 Not Found",
			"invalid_request",
		} {
			require.True(t, isDeviceFlowUnsupported(errors.New(msg)),
				"%q should read as an instance too old for the device flow", msg)
		}
		require.False(t, isDeviceFlowUnsupported(errors.New("invalid_grant")),
			"a real authorization failure is not a missing device grant")
	})

	t.Run("and the fallback completes the login", func(t *testing.T) {
		srv := fakegitlab.New(t, fakegitlab.WithoutDeviceGrant())
		host, inst := fakeHost(srv)
		store := newTestStore(t)

		var notice strings.Builder
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()

		creds, err := Login(ctx, LoginOptions{
			Host: host, ClientID: "test-client", Method: MethodAuto,
			Instance: &inst, Store: store, HTTPClient: srv.Client(),
			Browser: approveInBrowser(t), Out: &notice,
		})
		require.NoError(t, err)
		require.Equal(t, "browser-access", creds.Token,
			"the browser flow should have produced the credential")
		require.Contains(t, notice.String(), "17.9",
			"the fallback should say which version the instance needs, not fail silently")
	})
}

// AUT-01.T3 — the one-time code expires before it is approved.
//
// Prevents: an indefinite poll loop. GitLab keeps a device code for five
// minutes and the user has to be told which code is the live one.
func TestAUT01T3_AnExpiredDeviceCodeIsExplained(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t, fakegitlab.WithExpiredDeviceCode())
	host, inst := fakeHost(srv)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	_, err := Login(ctx, LoginOptions{
		Host: host, ClientID: "test-client", Method: MethodDevice,
		Instance: &inst, Store: newTestStore(t), HTTPClient: srv.Client(),
		NoBrowser: true, Out: io.Discard,
	})
	require.Error(t, err, "an expired code must fail rather than hang")
	require.Contains(t, err.Error(), "expired")
	require.Contains(t, err.Error(), fakegitlab.UserCode,
		"the message should name the code that died")
	require.Contains(t, err.Error(), "labdash auth login",
		"the message should carry the way out")
}

// AUT-02.T1 — the loopback flow with PKCE.
//
// Prevents: a silent PKCE mismatch, which surfaces as an unexplained
// invalid_grant. The fake verifies the challenge for real, so a flow that
// forgot the verifier fails here.
func TestAUT02T1_BrowserFlowExchangesThePKCEVerifier(t *testing.T) {
	srv := fakegitlab.New(t)
	host, inst := fakeHost(srv)
	store := newTestStore(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	creds, err := Login(ctx, LoginOptions{
		Host: host, ClientID: "test-client", Method: MethodBrowser,
		Instance: &inst, Store: store, HTTPClient: srv.Client(),
		Browser: approveInBrowser(t), Out: io.Discard,
	})
	require.NoError(t, err)
	require.Equal(t, "browser-access", creds.Token)
	require.True(t, creds.Managed)

	stored, err := store.Load(host)
	require.NoError(t, err)
	require.Equal(t, "browser-refresh", stored.RefreshToken)
}

// AUT-02.T2 — port 7171 is already taken.
//
// Prevents: an unexplained bind failure. The likeliest cause is a `glab auth
// login` still waiting, because glab uses the same port, and the message says
// so rather than printing a syscall error.
func TestAUT02T2_APortClashIsNamed(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:7171")
	if err != nil {
		t.Skipf("port 7171 is already in use by something else: %v", err)
	}
	defer listener.Close()

	_, err = browserFlow(t.Context(), "http://gitlab.example.com", "test-client",
		[]string{"api"}, func(string) error { return nil })
	require.Error(t, err)

	msg := err.Error()
	require.Contains(t, msg, "7171")
	require.Contains(t, msg, "glab", "the likeliest cause is worth naming")
	require.Contains(t, msg, "device flow", "the way out is the flow that needs no listener")
}

// AUT-03.T1 — a valid personal access token.
//
// Prevents: storing a typo and failing on the first dashboard refresh instead
// of at login, which is where the user can still fix it.
func TestAUT03T1_APastedTokenIsValidatedBeforeItIsStored(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t, fakegitlab.WithUser("tanuki", "Tanuki Example", "tanuki@example.com"))
	host, inst := fakeHost(srv)
	store := newTestStore(t)

	creds, err := LoginWithToken(t.Context(), "  glpat-example-token  ", LoginOptions{
		Host: host, Instance: &inst, Store: store, HTTPClient: srv.Client(),
	})
	require.NoError(t, err)
	require.Equal(t, "glpat-example-token", creds.Token, "surrounding whitespace should be trimmed")
	require.Equal(t, KindPAT, creds.Kind)

	require.Positive(t, srv.Calls("user"), "the token must be checked against the instance first")

	stored, err := store.Load(host)
	require.NoError(t, err)
	require.Equal(t, "glpat-example-token", stored.AccessToken)
}

// AUT-03.T2 — an invalid personal access token.
//
// Prevents: a bad credential poisoning the store, so that every later command
// fails against something the user believes they set correctly.
func TestAUT03T2_ARejectedTokenIsNeverStored(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t, fakegitlab.WithFailure("user", fakegitlab.Unauthorized))
	host, inst := fakeHost(srv)
	store := newTestStore(t)

	_, err := LoginWithToken(t.Context(), "glpat-wrong", LoginOptions{
		Host: host, Instance: &inst, Store: store, HTTPClient: srv.Client(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "that token did not work")
	require.Contains(t, err.Error(), "401")
	require.NotContains(t, err.Error(), "glpat-wrong", "an error must never carry the value")

	_, err = store.Load(host)
	require.ErrorIs(t, err, ErrNoStoredToken, "a rejected token must leave the store untouched")
}

// AUT-03.T2, second half — an empty token is refused before any request.
func TestAUT03T2_AnEmptyTokenIsRefusedLocally(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t)
	host, inst := fakeHost(srv)

	_, err := LoginWithToken(t.Context(), "   \n", LoginOptions{
		Host: host, Instance: &inst, Store: newTestStore(t), HTTPClient: srv.Client(),
	})
	require.Error(t, err)
	require.Zero(t, srv.Calls("user"), "an empty token needs no round trip to reject")
}

// AUT-19.T1 — the instance grants less than the login asked for.
//
// Prevents: discovering read-only by failing an action. The scope is recorded
// with the credential, so the dashboard knows before the first 403.
func TestAUT19T1_GrantedScopesAreRecordedAtLogin(t *testing.T) {
	t.Parallel()

	t.Run("oauth", func(t *testing.T) {
		t.Parallel()

		srv := fakegitlab.New(t, fakegitlab.WithGrantedScopes("read_api", "read_user"))
		host, inst := fakeHost(srv)
		store := newTestStore(t)

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()

		creds, err := Login(ctx, LoginOptions{
			Host: host, ClientID: "test-client", Method: MethodDevice,
			Instance: &inst, Store: store, HTTPClient: srv.Client(),
			NoBrowser: true, Out: io.Discard,
		})
		require.NoError(t, err)
		require.Equal(t, []string{"read_api", "read_user"}, creds.Scopes,
			"login asked for api and was handed read_api; that has to be recorded")

		stored, err := store.Load(host)
		require.NoError(t, err)
		require.Equal(t, []string{"read_api", "read_user"}, stored.Scopes,
			"the scope must survive a restart, or the next launch guesses again")
	})

	t.Run("personal access token", func(t *testing.T) {
		t.Parallel()

		srv := fakegitlab.New(t, fakegitlab.WithTokenScopes("read_api"))
		host, inst := fakeHost(srv)

		creds, err := LoginWithToken(t.Context(), "glpat-read-only", LoginOptions{
			Host: host, Instance: &inst, Store: newTestStore(t), HTTPClient: srv.Client(),
		})
		require.NoError(t, err)
		require.Equal(t, []string{"read_api"}, creds.Scopes)
	})

	t.Run("an instance that will not say leaves it unknown", func(t *testing.T) {
		t.Parallel()

		srv := fakegitlab.New(t, fakegitlab.WithFailure("token.self", fakegitlab.Forbidden))
		host, inst := fakeHost(srv)

		creds, err := LoginWithToken(t.Context(), "glpat-opaque", LoginOptions{
			Host: host, Instance: &inst, Store: newTestStore(t), HTTPClient: srv.Client(),
		})
		require.NoError(t, err, "an instance that cannot introspect must still allow a login")
		require.Empty(t, creds.Scopes)
	})
}

// approveInBrowser stands in for a person clicking Authorize: it follows the
// authorization URL, which the fake answers with the redirect back to the
// callback listener.
func approveInBrowser(t *testing.T) func(string) error {
	t.Helper()

	return func(url string) error {
		resp, err := http.Get(url) //nolint:noctx // stands in for a browser
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
}
