package gitlabauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Identity is who a credential belongs to — enough to greet the user and to
// show in `auth status`.
type Identity struct {
	Username  string
	Name      string
	Email     string
	AvatarURL string
	WebURL    string
}

// Greeting is a short, friendly line for startup. It degrades gracefully:
// a name if we have one, otherwise the handle, otherwise nothing at all.
func (i Identity) Greeting() string {
	switch {
	case i.Name != "" && i.Username != "":
		return fmt.Sprintf("%s (@%s)", i.Name, i.Username)
	case i.Username != "":
		return "@" + i.Username
	default:
		return i.Name
	}
}

// WhoAmI returns the identity behind a credential.
//
// This uses REST rather than GraphQL on purpose. GraphQL's
// `currentUser { email }` has been deprecated since GitLab 13.7 — it is an
// alias for publicEmail, which most users never set, so it comes back empty.
// `GET /user` returns the real address alongside the name and handle in a
// single request, provided the token carries read_user or api.
//
// A failure here is never fatal to the caller: the worst case is a greeting
// without a name.
func WhoAmI(ctx context.Context, creds Credentials, httpClient *http.Client) (Identity, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, creds.RESTEndpoint()+"/user", nil)
	if err != nil {
		return Identity{}, fmt.Errorf("building whoami request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+creds.Token)
	for _, h := range creds.CustomHeaders {
		req.Header.Set(h.Name, h.Resolve())
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return Identity{}, fmt.Errorf("whoami request to %s: %w", creds.Host, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Identity{}, fmt.Errorf("reading whoami response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return Identity{}, fmt.Errorf(
			"the credential for %s was rejected (HTTP 401)", creds.Host)
	case resp.StatusCode == http.StatusForbidden:
		return Identity{}, fmt.Errorf(
			"the credential for %s lacks the read_user scope (HTTP 403)", creds.Host)
	case resp.StatusCode >= 400:
		return Identity{}, fmt.Errorf("whoami: HTTP %d", resp.StatusCode)
	}

	user := struct {
		Username  string `json:"username"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
		WebURL    string `json:"web_url"`
	}{}
	if err := json.Unmarshal(body, &user); err != nil {
		return Identity{}, fmt.Errorf("decoding whoami response: %w", err)
	}

	if user.Username == "" && user.Name == "" {
		return Identity{}, fmt.Errorf("the credential for %s is not authenticating", creds.Host)
	}

	return Identity{
		Username:  user.Username,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		WebURL:    user.WebURL,
	}, nil
}
