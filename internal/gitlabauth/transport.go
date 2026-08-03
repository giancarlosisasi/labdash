package gitlabauth

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// requestTimeout bounds one request. It is deliberately shorter than any flow
// that wraps it: a corporate proxy that black-holes a connection must fail with
// a message rather than hang until the user gives up.
const requestTimeout = 30 * time.Second

// HTTPClient builds the client for this instance, carrying its private CA, its
// client certificate, its proxy and its custom headers.
//
// One client per instance rather than one per process: a corporate instance
// with a private CA and a proxy must not force its CA pool onto gitlab.com, and
// an instance that is unreachable must degrade only its own rows.
func (c Credentials) HTTPClient() (*http.Client, error) {
	tlsConfig, err := c.tlsConfig()
	if err != nil {
		return nil, err
	}

	proxy, err := c.proxyFunc()
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	transport.Proxy = proxy

	return &http.Client{
		Timeout:   requestTimeout,
		Transport: headerTransport{base: transport, headers: c.CustomHeaders},
	}, nil
}

func (c Credentials) tlsConfig() (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}

	// InsecureSkipVerify is a setting a corporate user genuinely needs and it is
	// theirs to choose, so it is honoured rather than argued with. `settings
	// show` is where it is called out.
	if c.SkipTLSVerify {
		cfg.InsecureSkipVerify = true
	}

	if c.CACert != "" {
		pem, err := os.ReadFile(c.CACert)
		if err != nil {
			return nil, fmt.Errorf("reading caCert for %s: %w", c.Host, err)
		}
		// Adding to the system pool rather than replacing it: an instance behind
		// a private CA still redirects to public hosts for avatars and docs.
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("caCert for %s holds no certificate: %s", c.Host, c.CACert)
		}
		cfg.RootCAs = pool
	}

	switch {
	case c.ClientCert != "" && c.ClientKey != "":
		pair, err := tls.LoadX509KeyPair(c.ClientCert, c.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("reading the client certificate for %s: %w", c.Host, err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	case c.ClientCert != "":
		return nil, fmt.Errorf("clientCert is set for %s but clientKey is not; "+
			"a certificate without its key cannot be presented", c.Host)
	case c.ClientKey != "":
		return nil, fmt.Errorf("clientKey is set for %s but clientCert is not", c.Host)
	}

	return cfg, nil
}

// proxyFunc returns the instance's own proxy, or the environment's.
//
// An explicit `proxy:` wins over HTTPS_PROXY, because a user who names one for
// this instance has said something more specific than the shell has.
func (c Credentials) proxyFunc() (func(*http.Request) (*url.URL, error), error) {
	if c.Proxy == "" {
		return http.ProxyFromEnvironment, nil
	}

	parsed, err := url.Parse(c.Proxy)
	if err != nil {
		return nil, fmt.Errorf("parsing the proxy for %s: %w", c.Host, err)
	}

	return http.ProxyURL(parsed), nil
}

// headerTransport adds the instance's custom headers to every request. Auth
// proxies such as Cloudflare Access reject anything without them, including the
// login itself.
type headerTransport struct {
	base    http.RoundTripper
	headers []Header
}

func (t headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(t.headers) == 0 {
		return t.base.RoundTrip(req)
	}

	// RoundTrip must not modify the request it is given.
	clone := req.Clone(req.Context())
	for _, h := range t.headers {
		if v := h.Resolve(); v != "" {
			clone.Header.Set(h.Name, v)
		}
	}

	return t.base.RoundTrip(clone)
}
