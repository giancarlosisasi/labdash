package gitlabauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// AUT-16.T1 — an instance with a private CA, a client certificate and a proxy.
//
// Prevents: a corporate user with a private CA being unable to connect at all.
// Every one of these is per instance, so a private CA on one host never has to
// be trusted on another.
func TestAUT16T1_TheClientCarriesTheInstancesTLSAndProxySettings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	caPath := filepath.Join(dir, "corp-root.pem")
	certPath := filepath.Join(dir, "client.pem")
	keyPath := filepath.Join(dir, "client-key.pem")
	writeSelfSignedPair(t, caPath, certPath, keyPath)

	creds := Credentials{
		Host:       "gitlab.example.com",
		CACert:     caPath,
		ClientCert: certPath,
		ClientKey:  keyPath,
		Proxy:      "http://proxy.internal:3128",
	}

	client, err := creds.HTTPClient()
	require.NoError(t, err)

	transport := unwrap(t, client)
	require.NotNil(t, transport.TLSClientConfig.RootCAs, "the private CA was dropped")
	require.Len(t, transport.TLSClientConfig.Certificates, 1, "the client certificate was dropped")

	proxied, err := transport.Proxy(mustRequest(t, "https://gitlab.example.com/api/v4/user"))
	require.NoError(t, err)
	require.Equal(t, "proxy.internal:3128", proxied.Host, "the instance's proxy was dropped")
}

// AUT-16.T1, second half — the environment's proxy is honoured, and an explicit
// one for the instance overrides it.
//
// Prevents: a user who set HTTPS_PROXY finding that only labdash ignores it,
// and a user who named a proxy for one instance finding the shell wins anyway.
func TestAUT16T1_ProxyPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("the environment is honoured when the instance names none", func(t *testing.T) {
		t.Parallel()

		client, err := Credentials{Host: "gitlab.example.com"}.HTTPClient()
		require.NoError(t, err)

		// Identity rather than behaviour, because net/http reads the
		// environment once per process and a test cannot move it afterwards.
		// Delegating to the standard reader is the whole of the promise:
		// HTTPS_PROXY and NO_PROXY then work exactly as they do everywhere
		// else, including the forms nobody remembers.
		require.Equal(t,
			reflect.ValueOf(http.ProxyFromEnvironment).Pointer(),
			reflect.ValueOf(unwrap(t, client).Proxy).Pointer(),
			"HTTPS_PROXY and NO_PROXY are ignored unless we delegate to net/http")
	})

	t.Run("an explicit proxy for the instance wins", func(t *testing.T) {
		t.Parallel()

		client, err := Credentials{
			Host: "gitlab.example.com", Proxy: "http://named-for-this-instance:3128",
		}.HTTPClient()
		require.NoError(t, err)

		proxied, err := unwrap(t, client).Proxy(mustRequest(t, "https://gitlab.example.com"))
		require.NoError(t, err)
		require.Equal(t, "named-for-this-instance:3128", proxied.Host,
			"a proxy named for this instance is more specific than the shell's")
	})
}

// AUT-16.T2 — customHeaders, including the environment indirection.
//
// Prevents: a Cloudflare-Access secret in a log file, and an auth proxy
// rejecting every request because the header never left our transport.
func TestAUT16T2_CustomHeadersReachTheWireAndTheSecretStaysOutOfTheFile(t *testing.T) {
	t.Setenv("CF_ACCESS_SECRET", "secret-from-the-environment")

	var seen http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	creds := Credentials{
		Host: "gitlab.example.com",
		CustomHeaders: []Header{
			{Name: "Cf-Access-Client-Secret", ValueFromEnv: "CF_ACCESS_SECRET"},
			{Name: "X-Static", Value: "literal"},
			{Name: "X-Missing", ValueFromEnv: "NOT_SET_ANYWHERE"},
		},
	}

	client, err := creds.HTTPClient()
	require.NoError(t, err)

	resp, err := client.Do(mustRequest(t, srv.URL))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, "secret-from-the-environment", seen.Get("Cf-Access-Client-Secret"))
	require.Equal(t, "literal", seen.Get("X-Static"))
	require.Empty(t, seen.Get("X-Missing"), "an unset variable must not send an empty header")

	// The settings file records the variable's name, never its value. That is
	// what keeps the file safe to commit or to hand to a colleague.
	require.NotContains(t, creds.String(), "secret-from-the-environment")
}

// A certificate without its key, or the reverse, is a misconfiguration worth
// naming at startup rather than as a handshake failure later.
func TestAUT16T1_AHalfConfiguredClientCertificateIsRefused(t *testing.T) {
	t.Parallel()

	_, err := Credentials{Host: "gitlab.example.com", ClientCert: "/tmp/only-the-cert.pem"}.HTTPClient()
	require.ErrorContains(t, err, "clientKey")

	_, err = Credentials{Host: "gitlab.example.com", ClientKey: "/tmp/only-the-key.pem"}.HTTPClient()
	require.ErrorContains(t, err, "clientCert")
}

// A caCert that is not there, or is not a certificate, is named with the host
// it belongs to.
func TestAUT16T1_AnUnreadableCACertIsNamed(t *testing.T) {
	t.Parallel()

	_, err := Credentials{Host: "gitlab.example.com", CACert: "/nowhere/corp.pem"}.HTTPClient()
	require.ErrorContains(t, err, "gitlab.example.com")

	empty := filepath.Join(t.TempDir(), "not-a-cert.pem")
	require.NoError(t, os.WriteFile(empty, []byte("this is not a certificate"), 0o600))

	_, err = Credentials{Host: "gitlab.example.com", CACert: empty}.HTTPClient()
	require.ErrorContains(t, err, "no certificate")
}

func unwrap(t *testing.T, client *http.Client) *http.Transport {
	t.Helper()

	wrapper, ok := client.Transport.(headerTransport)
	require.True(t, ok, "the custom-header transport is missing")

	transport, ok := wrapper.base.(*http.Transport)
	require.True(t, ok)
	return transport
}

func mustRequest(t *testing.T, url string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	return req
}

// writeSelfSignedPair writes one self-signed certificate three times over: as a
// CA bundle, as a client certificate, and as its key. The transport only has to
// load them, not trust anything they say.
func writeSelfSignedPair(t *testing.T, caPath, certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "labdash test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	require.NoError(t, os.WriteFile(caPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))
}
