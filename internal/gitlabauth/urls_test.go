package gitlabauth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// AUT-17.T1 and REG-08 — every URL labdash builds for a subfolder install.
//
// Prevents: the exact bug already found once. A hardcoded https and a missing
// subfolder in the OAuth URLs, which works on gitlab.com and on nothing else.
// The API may be proxied to another hostname; the OAuth endpoints never are,
// because they are pages a browser visits.
func TestAUT17T1_REG08_URLsHonourProtocolSubfolderAndTheWebHost(t *testing.T) {
	t.Parallel()

	subfolder := Credentials{
		Host:        "example.com",
		APIHost:     "api.example.com",
		APIProtocol: "http",
		Subfolder:   "gitlab",
	}

	require.Equal(t, "http://api.example.com/gitlab/api/graphql", subfolder.GraphQLEndpoint())
	require.Equal(t, "http://api.example.com/gitlab/api/v4", subfolder.RESTEndpoint())
	require.Equal(t, "http://example.com/gitlab", subfolder.oauthBaseURL(),
		"OAuth endpoints hang off the web host, never off apiHost")

	t.Run("a leading or trailing slash on the subfolder is tolerated", func(t *testing.T) {
		t.Parallel()

		messy := Credentials{Host: "example.com", APIHost: "example.com", Subfolder: "/gitlab/"}
		require.Equal(t, "https://example.com/gitlab/api/graphql", messy.GraphQLEndpoint())
		require.Equal(t, "https://example.com/gitlab", messy.oauthBaseURL())
	})

	t.Run("gitlab.com uses the library's built-in endpoints", func(t *testing.T) {
		t.Parallel()

		require.Empty(t, Credentials{Host: DefaultHost, APIProtocol: "https"}.oauthBaseURL(),
			"an empty base URL is what makes gitlaboauth2 use its own endpoints")
	})

	t.Run("a subfolder on gitlab.com is still a subfolder", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, "https://gitlab.com/sub",
			Credentials{Host: DefaultHost, APIProtocol: "https", Subfolder: "sub"}.oauthBaseURL())
	})
}

// REG-08, second half — the refresh exchange goes to the instance's own token
// endpoint rather than to gitlab.com's.
func TestREG08_RefreshUsesTheInstancesOwnTokenEndpoint(t *testing.T) {
	t.Parallel()

	creds := Credentials{
		Host: "example.com", APIHost: "api.example.com",
		APIProtocol: "http", Subfolder: "gitlab",
	}
	require.Equal(t, "http://example.com/gitlab", creds.oauthBaseURL(),
		"a refresh built from apiHost or from https would reach the wrong server")
}
