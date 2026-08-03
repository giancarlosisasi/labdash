package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/giancarlosisasi/labdash/internal/action"
	"github.com/giancarlosisasi/labdash/internal/gitlabauth"
	"github.com/giancarlosisasi/labdash/internal/tui/onboarding"
)

// signIn is the credential layer as the wizard sees it.
//
// It lives here rather than in internal/tui because the wizard must not depend
// on the auth package: the boundary is what lets the wizard be driven whole in
// a test with no network and no keyring.
type signIn struct {
	store *gitlabauth.Store

	mu sync.Mutex
	// pending is the device authorization StartDevice began, held so that
	// AwaitApproval can finish the same one rather than starting a second.
	pending *gitlabauth.DeviceAuth
}

func newSignIn(d deps) *signIn { return &signIn{store: d.Store} }

// options builds the login options for a host from the settings file, so a
// self-managed instance's clientId and transport settings apply to the login
// itself and not only to what comes after it.
func (s *signIn) options(host string) (gitlabauth.LoginOptions, error) {
	cfg, err := gitlabauth.LoadConfig()
	if err != nil {
		return gitlabauth.LoginOptions{}, err
	}
	inst, _ := cfg.Instance(host)

	return gitlabauth.LoginOptions{
		Host:     host,
		Instance: &inst,
		Store:    s.store,
	}, nil
}

// Known reports whether the wizard should skip the offer to save this host.
//
// gitlab.com is always known. It is the built-in instance: it needs no client
// id and no transport settings, so an entry for it would hold nothing. Offering
// to write one is the thing CFG-23 exists to prevent — gitlab.com works with no
// settings file at all, and a fresh user must not be asked to create one.
func (s *signIn) Known(host string) bool {
	if host == gitlabauth.DefaultHost {
		return true
	}

	cfg, err := gitlabauth.LoadConfig()
	if err != nil {
		return false
	}

	_, ok := cfg.Instance(host)
	return ok
}

func (s *signIn) StartDevice(ctx context.Context, host string) (onboarding.Code, error) {
	opts, err := s.options(host)
	if err != nil {
		return onboarding.Code{}, err
	}

	auth, err := gitlabauth.DeviceStart(ctx, opts)
	if err != nil {
		return onboarding.Code{}, err
	}

	s.mu.Lock()
	s.pending = auth
	s.mu.Unlock()

	return onboarding.Code{
		UserCode: auth.UserCode,
		URL:      auth.VerificationURL,
		Expires:  auth.Expiry,
	}, nil
}

func (s *signIn) AwaitApproval(ctx context.Context, host string) (onboarding.Account, error) {
	s.mu.Lock()
	auth := s.pending
	s.mu.Unlock()

	if auth == nil {
		return onboarding.Account{}, errors.New("there is no code waiting for approval")
	}

	opts, err := s.options(host)
	if err != nil {
		return onboarding.Account{}, err
	}

	creds, err := gitlabauth.DeviceComplete(ctx, auth, opts)
	if err != nil {
		return onboarding.Account{}, err
	}

	return s.account(ctx, creds), nil
}

func (s *signIn) DeviceUnsupported(err error) bool {
	return gitlabauth.IsDeviceFlowUnsupported(err)
}

func (s *signIn) LoginWithBrowser(ctx context.Context, host string) (onboarding.Account, error) {
	opts, err := s.options(host)
	if err != nil {
		return onboarding.Account{}, err
	}

	creds, err := gitlabauth.BrowserLogin(ctx, opts)
	if err != nil {
		return onboarding.Account{}, err
	}

	return s.account(ctx, creds), nil
}

func (s *signIn) LoginWithToken(ctx context.Context, host, token string) (onboarding.Account, error) {
	opts, err := s.options(host)
	if err != nil {
		return onboarding.Account{}, err
	}

	creds, err := gitlabauth.LoginWithToken(ctx, token, opts)
	if err != nil {
		return onboarding.Account{}, err
	}

	return s.account(ctx, creds), nil
}

// SaveInstance writes the host's stanza and nothing else. The defaults are
// correct for a plain instance; what the entry buys the user is somewhere to
// put caCert, clientCert, proxy and subfolder.
func (s *signIn) SaveInstance(host string) error {
	if err := gitlabauth.SaveInstance(host, gitlabauth.InstanceConfig{}); err != nil {
		return fmt.Errorf("writing %s: %w", gitlabauth.SettingsPath(), err)
	}
	return nil
}

func (s *signIn) OpenURL(url string) error { return gitlabauth.OpenBrowser(url) }

// account names the credential's owner and what it may do. Failing to identify
// the user is not a failed login: the worst case is a dashboard with no name in
// its context bar.
func (s *signIn) account(ctx context.Context, creds gitlabauth.Credentials) onboarding.Account {
	acct := onboarding.Account{
		Host:  creds.Host,
		Scope: action.ScopeFromGranted(creds.Scopes),
	}
	if id, err := gitlabauth.WhoAmI(ctx, creds, nil); err == nil {
		acct.Username = id.Username
	}
	return acct
}
