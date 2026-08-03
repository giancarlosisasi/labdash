package gitlabauth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// A FailureKind is the two ways a credential stops working. Everything else is
// a network problem or a bug, and neither is fixed by signing in again.
type FailureKind int

const (
	// NotAFailure means the response was not about the credential at all.
	NotAFailure FailureKind = iota
	// Unauthenticated is a 401: the token is expired, revoked, or wrong.
	Unauthenticated
	// InsufficientScope is a 403 caused by the token's scope rather than by the
	// user's permissions on the object.
	InsufficientScope
)

// A StatusError is a refusal from an instance, carrying the status and the body
// so a caller can classify it rather than matching on a sentence.
//
// Its own message is still written for a person, because it reaches the CLI
// unchanged: only the dashboard has somewhere better to put it.
type StatusError struct {
	Status int
	Host   string
	Body   string
}

func (e *StatusError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized:
		return fmt.Sprintf("the credential for %s was rejected (HTTP 401)", e.Host)
	case http.StatusForbidden:
		return fmt.Sprintf("the credential for %s is not allowed to do that (HTTP 403)", e.Host)
	default:
		return fmt.Sprintf("%s answered HTTP %d", e.Host, e.Status)
	}
}

// A Failure is an authentication problem rewritten for the person at the
// keyboard.
//
// It carries the instance, because a two-instance user needs to know which one
// stopped working, and the instance's own token URL, because sending a
// self-managed user to gitlab.com's settings page is worse than saying nothing.
type Failure struct {
	Kind FailureKind
	Host string
	// TokenURL is where this instance mints a personal access token, built from
	// its own protocol and subfolder.
	TokenURL string
}

// Classify reads a response and reports whether it is about the credential.
//
// The body matters for 403: GitLab answers both "your token may not do this"
// and "you may not do this to that object" with 403, and only the first is
// fixed by making a new token.
// ClassifyError is Classify over an error returned by this package, for a
// caller that has an error rather than a response in its hand.
func ClassifyError(creds Credentials, err error) (Failure, bool) {
	var status *StatusError
	if !errors.As(err, &status) {
		return Failure{}, false
	}
	return Classify(creds, status.Status, status.Body)
}

func Classify(creds Credentials, status int, body string) (Failure, bool) {
	f := Failure{Host: creds.Host, TokenURL: creds.TokenURL()}

	switch {
	case status == http.StatusUnauthorized:
		f.Kind = Unauthenticated
	case status == http.StatusForbidden && mentionsScope(body):
		f.Kind = InsufficientScope
	default:
		return Failure{}, false
	}

	return f, true
}

// mentionsScope reports whether a 403 blames the token's scope. GitLab words it
// several ways, so the match is on the words that appear in all of them.
func mentionsScope(body string) bool {
	lower := strings.ToLower(body)
	for _, phrase := range []string{
		"insufficient_scope",
		"insufficient scope",
		"scope is not allowed",
		"missing scope",
		"read_api",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// TokenURL is where this instance mints a personal access token, with the api
// scope pre-selected.
//
// It hangs off the web host and honours the subfolder, exactly as the OAuth
// endpoints do: a user following this link is opening a page, not calling an
// API, and only the API is ever proxied elsewhere.
func (c Credentials) TokenURL() string {
	base := oauthBaseURL(c.APIProtocol, c.Host, c.Subfolder)
	if base == "" {
		base = "https://" + DefaultHost
	}
	return base + "/-/user_settings/personal_access_tokens?scopes=api"
}

// Message is the whole of what the user reads: what failed, why, and what to
// do about it.
func (f Failure) Message(recoveryKey string) string {
	switch f.Kind {
	case Unauthenticated:
		return fmt.Sprintf(
			"%s rejected the credential — it has expired or been revoked. "+
				"Press %s to sign in again, or create a token at %s",
			f.Host, recoveryKey, f.TokenURL)

	case InsufficientScope:
		return fmt.Sprintf(
			"%s refused that — this token is read-only. "+
				"Create one with the api scope at %s, then press %s to sign in again",
			f.Host, f.TokenURL, recoveryKey)

	default:
		return ""
	}
}
