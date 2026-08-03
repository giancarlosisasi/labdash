package fakegitlab

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The device flow's two codes are fixed rather than random: a test that asserts
// on the code a user is asked to type needs to know what it will be.
const (
	DeviceCode = "fake-device-code"
	UserCode   = "K7QW-3FDB"
)

// oauthScenario is everything the fake's OAuth surface can be told to do.
type oauthScenario struct {
	// deviceGrant off makes /oauth/authorize_device answer as an instance
	// older than GitLab 17.9 does, which is what the browser fallback exists
	// for.
	deviceGrant bool
	// devicePolls is how many authorization_pending answers precede the token.
	devicePolls int
	// deviceExpired answers expired_token instead of ever issuing one.
	deviceExpired bool
	// granted is the scope string the token response reports, which may be
	// narrower than what the login asked for.
	granted []string
	// patScopes is what GET /personal_access_tokens/self reports.
	patScopes []string

	username, name, email string
}

func defaultOAuth() oauthScenario {
	return oauthScenario{
		deviceGrant: true,
		granted:     []string{"api", "read_user"},
		patScopes:   []string{"api"},
		username:    "tanuki",
		name:        "Tanuki Example",
		email:       "tanuki@example.com",
	}
}

// WithGrantedScopes sets the scopes the token response reports as granted.
// Asking for `api` and being handed `read_api` is the case AUT-19 exists for.
func WithGrantedScopes(scopes ...string) Option {
	return func(s *Server) { s.oauth.granted = scopes }
}

// WithoutDeviceGrant makes the instance reject RFC 8628, as every GitLab older
// than 17.9 does.
func WithoutDeviceGrant() Option {
	return func(s *Server) { s.oauth.deviceGrant = false }
}

// WithDevicePolls makes the device flow answer authorization_pending n times
// before it issues a token, so a test can watch the polling interval hold.
func WithDevicePolls(n int) Option {
	return func(s *Server) { s.oauth.devicePolls = n }
}

// WithExpiredDeviceCode never issues a token: the code lapses, which is the
// five-minute failure a real user hits.
func WithExpiredDeviceCode() Option {
	return func(s *Server) { s.oauth.deviceExpired = true }
}

// WithTokenScopes sets what GET /personal_access_tokens/self reports, which is
// how a pasted token's scopes are discovered.
func WithTokenScopes(scopes ...string) Option {
	return func(s *Server) { s.oauth.patScopes = scopes }
}

// WithUser sets who GET /user reports.
func WithUser(username, name, email string) Option {
	return func(s *Server) {
		s.oauth.username, s.oauth.name, s.oauth.email = username, name, email
	}
}

func (s *Server) authorizeDevice(w http.ResponseWriter, r *http.Request) {
	s.record(Request{Method: r.Method, Path: r.URL.Path, Operation: "oauth.device", Header: r.Header.Clone()})

	if s.stall(w, r, "oauth.device") {
		return
	}

	if !s.oauth.deviceGrant {
		s.writeJSON(w, http.StatusNotFound, map[string]any{
			"error":             "unsupported_grant_type",
			"error_description": "the device authorization grant is not enabled",
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"device_code":               DeviceCode,
		"user_code":                 UserCode,
		"verification_uri":          s.URL + "/oauth/device",
		"verification_uri_complete": s.URL + "/oauth/device?user_code=" + UserCode,
		"expires_in":                300,
		"interval":                  1,
	})
}

// authorize is the browser flow's first leg. It answers the redirect a real
// GitLab answers once the user has approved, so the callback server, the state
// check and the PKCE exchange all run for real.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) {
	s.record(Request{Method: r.Method, Path: r.URL.Path, Operation: "oauth.authorize", Header: r.Header.Clone()})

	q := r.URL.Query()
	redirect, err := url.Parse(q.Get("redirect_uri"))
	if err != nil || redirect.Host == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}

	code := fmt.Sprintf("fake-auth-code-%d", len(s.Requests()))
	s.mu.Lock()
	s.challenges[code] = q.Get("code_challenge")
	s.mu.Unlock()

	params := redirect.Query()
	params.Set("code", code)
	params.Set("state", q.Get("state"))
	redirect.RawQuery = params.Encode()

	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}

	grant := r.PostForm.Get("grant_type")
	s.record(Request{
		Method: r.Method, Path: r.URL.Path, Operation: "oauth.token",
		Variables: map[string]any{"grant_type": grant},
		Header:    r.Header.Clone(),
	})

	if s.stall(w, r, "oauth.token") {
		return
	}

	switch grant {
	case "urn:ietf:params:oauth:grant-type:device_code":
		s.deviceToken(w)
	case "authorization_code":
		s.codeToken(w, r)
	case "refresh_token":
		s.issueToken(w, "refreshed-access", "refreshed-refresh")
	default:
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
	}
}

func (s *Server) deviceToken(w http.ResponseWriter) {
	if s.oauth.deviceExpired {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":             "expired_token",
			"error_description": "the device code has expired",
		})
		return
	}

	s.mu.Lock()
	pending := s.pending
	if pending > 0 {
		s.pending--
	}
	s.mu.Unlock()

	if pending > 0 {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "authorization_pending"})
		return
	}

	s.issueToken(w, "device-access", "device-refresh")
}

func (s *Server) codeToken(w http.ResponseWriter, r *http.Request) {
	code := r.PostForm.Get("code")

	s.mu.Lock()
	challenge, known := s.challenges[code]
	s.mu.Unlock()

	if !known {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	// Verifying the challenge for real is the point. A fake that accepts any
	// verifier would pass a client that never sends one, which is precisely the
	// PKCE mistake that shows up later as an unexplained invalid_grant.
	if challenge != "" && challenge != s256(r.PostForm.Get("code_verifier")) {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":             "invalid_grant",
			"error_description": "the PKCE verifier does not match the challenge",
		})
		return
	}

	s.issueToken(w, "browser-access", "browser-refresh")
}

func (s *Server) issueToken(w http.ResponseWriter, access, refresh string) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
		"token_type":    "Bearer",
		"expires_in":    int((2 * time.Hour).Seconds()),
		"scope":         strings.Join(s.oauth.granted, " "),
		"created_at":    1,
	})
}

func (s *Server) currentUser(w http.ResponseWriter, r *http.Request) {
	s.record(Request{Method: r.Method, Path: r.URL.Path, Operation: "user", Header: r.Header.Clone()})

	if s.stall(w, r, "user") {
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":       1,
		"username": s.oauth.username,
		"name":     s.oauth.name,
		"email":    s.oauth.email,
		"web_url":  s.URL + "/" + s.oauth.username,
	})
}

func (s *Server) tokenSelf(w http.ResponseWriter, r *http.Request) {
	s.record(Request{Method: r.Method, Path: r.URL.Path, Operation: "token.self", Header: r.Header.Clone()})

	if s.stall(w, r, "token.self") {
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":     1,
		"name":   "labdash",
		"scopes": s.oauth.patScopes,
		"active": true,
	})
}

func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
