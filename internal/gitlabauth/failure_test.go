package gitlabauth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// AUT-08.T1 — a mutation refused for insufficient scope.
//
// Prevents: a raw GraphQL error dump. The user is told the token is read-only
// and which scope to create, which is the only thing that fixes it.
func TestAUT08T1_AScopeFailureNamesTheScopeToCreate(t *testing.T) {
	t.Parallel()

	creds := Credentials{Host: "gitlab.example.com", APIHost: "gitlab.example.com", APIProtocol: "https"}
	body := `{"error":"insufficient_scope","error_description":"The request requires higher privileges"}`

	failure, ours := Classify(creds, http.StatusForbidden, body)
	require.True(t, ours, "a 403 blaming the scope is a credential problem")
	require.Equal(t, InsufficientScope, failure.Kind)

	msg := failure.Message("Ctrl+A")
	require.Contains(t, msg, "gitlab.example.com", "a two-instance user needs to know which one")
	require.Contains(t, msg, "read-only")
	require.Contains(t, msg, "api scope")
	require.Contains(t, msg, "https://gitlab.example.com/-/user_settings/personal_access_tokens")
	require.Contains(t, msg, "Ctrl+A")
	require.NotContains(t, msg, "insufficient_scope", "the raw error body reached the user")
	require.NotContains(t, msg, "error_description")
}

// AUT-08.T1, the other half — a 403 about the object rather than the token.
//
// Prevents: telling a user to make a new token when the token was never the
// problem. A maintainer who cannot merge a protected branch has the right scope
// and the wrong permission.
func TestAUT08T1_APermissionFailureIsNotAScopeFailure(t *testing.T) {
	t.Parallel()

	creds := Credentials{Host: "gitlab.com"}
	_, ours := Classify(creds, http.StatusForbidden,
		`{"message":"You are not allowed to merge into this branch"}`)
	require.False(t, ours, "a permissions 403 must not be rewritten as a scope problem")
}

// AUT-09.T1 — any request refused as unauthenticated.
//
// Prevents: a 401 dump with no path forward. The message names the instance,
// carries that instance's own token URL, and offers the key that re-runs login.
func TestAUT09T1_AnExpiredCredentialOffersAWayBack(t *testing.T) {
	t.Parallel()

	t.Run("gitlab.com", func(t *testing.T) {
		t.Parallel()

		failure, ours := Classify(
			Credentials{Host: "gitlab.com", APIProtocol: "https"},
			http.StatusUnauthorized, `{"message":"401 Unauthorized"}`)
		require.True(t, ours)
		require.Equal(t, Unauthenticated, failure.Kind)

		msg := failure.Message("Ctrl+A")
		require.Contains(t, msg, "gitlab.com")
		require.Contains(t, msg, "expired or been revoked")
		require.Contains(t, msg, "https://gitlab.com/-/user_settings/personal_access_tokens?scopes=api")
		require.Contains(t, msg, "Ctrl+A")
	})

	t.Run("a subfolder-hosted instance", func(t *testing.T) {
		t.Parallel()

		creds := Credentials{
			Host: "example.com", APIHost: "api.example.com",
			APIProtocol: "https", Subfolder: "gitlab",
		}

		failure, ours := Classify(creds, http.StatusUnauthorized, "401 Unauthorized")
		require.True(t, ours)

		msg := failure.Message("Ctrl+A")
		require.Contains(t, msg, "https://example.com/gitlab/-/user_settings/personal_access_tokens",
			"the token page is on the web host and inside the subfolder")
		require.NotContains(t, msg, "api.example.com",
			"only the API is ever proxied; a browser goes to the web host")
	})
}

// Anything that is not about the credential is left alone.
//
// Prevents: a rate limit or a server error being rewritten as "sign in again",
// which sends the user to fix something that was never broken.
func TestClassifyLeavesEverythingElseAlone(t *testing.T) {
	t.Parallel()

	for _, status := range []int{
		http.StatusOK, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusNotFound,
	} {
		_, ours := Classify(Credentials{Host: "gitlab.com"}, status, "")
		require.False(t, ours, "HTTP %d was treated as a credential failure", status)
	}
}
