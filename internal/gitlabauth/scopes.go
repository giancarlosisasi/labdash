package gitlabauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

// scopesFromToken reads the scopes an instance actually granted out of its token
// response.
//
// GitLab returns a space-separated `scope` field, and it may be narrower than
// what the login asked for — an application registered without `api` hands back
// `read_api` and nothing says so. Recording it here is what lets the dashboard
// know it is read-only before it tries to merge something.
func scopesFromToken(tok *oauth2.Token) []string {
	if tok == nil {
		return nil
	}
	raw, _ := tok.Extra("scope").(string)
	return strings.Fields(raw)
}

// TokenScopes asks the instance what a personal access token is allowed to do.
//
// A PAT carries no scope information of its own, so the only way to know before
// the first refused mutation is to ask. A failure is not fatal: an instance too
// old for the endpoint, or a token that cannot introspect itself, leaves the
// scopes unknown, and an unknown scope set is treated as writable.
func TokenScopes(ctx context.Context, creds Credentials, httpClient *http.Client) ([]string, error) {
	if httpClient == nil {
		var err error
		if httpClient, err = creds.HTTPClient(); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, creds.RESTEndpoint()+"/personal_access_tokens/self", nil)
	if err != nil {
		return nil, fmt.Errorf("building token-scopes request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+creds.Token)
	for _, h := range creds.CustomHeaders {
		req.Header.Set(h.Name, h.Resolve())
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token-scopes request to %s: %w", creds.Host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("the scopes of the token for %s could not be read (HTTP %d)",
			creds.Host, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading the token-scopes response: %w", err)
	}

	self := struct {
		Scopes []string `json:"scopes"`
	}{}
	if err := json.Unmarshal(body, &self); err != nil {
		return nil, fmt.Errorf("decoding the token-scopes response: %w", err)
	}

	return self.Scopes, nil
}
