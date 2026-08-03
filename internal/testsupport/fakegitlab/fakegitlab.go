// Package fakegitlab is an in-process GitLab, so that no unit test needs a
// real one.
//
// It answers POST /api/graphql, the four REST endpoints labdash uses, and
// GET /version. It is backed by a scenario struct rather than by recorded
// traffic, because latency, failure, rate limiting and pagination are the four
// things the tests most need to vary and none of them is expressible in a
// recorded response body. Bodies are still loaded from fixtures — for shape
// fidelity — but the behaviour is programmable.
//
// Rule: fakegitlab never imports internal/tui. A test that needs both is an
// integration test and wires them at the test boundary.
//
// See research/14-test-strategy.md §3.1.
package fakegitlab

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

//go:embed testdata/fixtures
var fixtures embed.FS

// A Failure is how an operation fails when it is told to.
type Failure int

const (
	// NoFailure is the zero value: the operation succeeds.
	NoFailure Failure = iota
	// RateLimited answers 429 with Retry-After, which is what PRF-05 backs off
	// from.
	RateLimited
	// Unauthorized answers 401, as an expired token does.
	Unauthorized
	// Forbidden answers 403, as a read-only token does on a mutation.
	Forbidden
	// ServerError answers 500.
	ServerError
	// Timeout never answers at all, which is what PRF-04's fetch budget has to
	// survive.
	Timeout
	// PartialResponse answers 200 with both data and an errors array, which is
	// GraphQL's way of half-failing and the case a naive client drops on the
	// floor.
	PartialResponse
)

func (f Failure) status() int {
	switch f {
	case RateLimited:
		return http.StatusTooManyRequests
	case Unauthorized:
		return http.StatusUnauthorized
	case Forbidden:
		return http.StatusForbidden
	case ServerError:
		return http.StatusInternalServerError
	default:
		return http.StatusOK
	}
}

// An Option programmes one aspect of the scenario.
type Option func(*Server)

// WithVersion sets what GET /version reports. The minimum-version checks in
// DIA-07 and MUL-08 read it.
func WithVersion(v string) Option {
	return func(s *Server) { s.version = v }
}

// WithLatency makes one operation take d before it answers. The operation name
// is the GraphQL operation name, or the REST route key such as "job.trace".
func WithLatency(operation string, d time.Duration) Option {
	return func(s *Server) { s.latency[operation] = d }
}

// WithFailure makes one operation fail in a named way.
func WithFailure(operation string, f Failure) Option {
	return func(s *Server) { s.failures[operation] = f }
}

// WithRetryAfter sets the Retry-After a RateLimited operation reports.
func WithRetryAfter(d time.Duration) Option {
	return func(s *Server) { s.retryAfter = d }
}

// WithFixture overrides the response body for one operation. The body is used
// verbatim, so a test can hand-write a shape the fixtures do not carry yet.
func WithFixture(operation string, body []byte) Option {
	return func(s *Server) { s.bodies[operation] = body }
}

// WithPages makes one operation paginate: each call returns the next page and
// reports hasNextPage until the pages run out.
func WithPages(operation string, pages [][]byte) Option {
	return func(s *Server) { s.pages[operation] = pages }
}

// A Server is a running fake GitLab. Close is registered with the test, so a
// caller never has to remember it.
type Server struct {
	*httptest.Server

	version    string
	retryAfter time.Duration

	latency  map[string]time.Duration
	failures map[string]Failure
	bodies   map[string][]byte
	pages    map[string][][]byte

	oauth oauthScenario

	mu       sync.Mutex
	requests []Request
	pageAt   map[string]int
	// pending counts the authorization_pending answers still owed on the
	// device flow, and challenges holds one PKCE challenge per issued code.
	pending    int
	challenges map[string]string
}

// A Request is one call the fake received, for a test that asserts on traffic
// rather than on the answer. DIA-06.T2 asserts the whole set of hosts contacted
// from this record.
type Request struct {
	Method    string
	Path      string
	Operation string
	Variables map[string]any
	Header    http.Header
}

// New starts a fake GitLab and registers its shutdown with t.
func New(t testing.TB, opts ...Option) *Server {
	t.Helper()

	s := &Server{
		version:    "17.9.0",
		retryAfter: 2 * time.Second,
		latency:    map[string]time.Duration{},
		failures:   map[string]Failure{},
		bodies:     map[string][]byte{},
		pages:      map[string][][]byte{},
		oauth:      defaultOAuth(),
		pageAt:     map[string]int{},
		challenges: map[string]string{},
	}
	for _, o := range opts {
		o(s)
	}
	s.pending = s.oauth.devicePolls

	s.Server = httptest.NewServer(s.handler())
	t.Cleanup(s.Server.Close)

	return s
}

// Requests returns every call the fake received, in order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Calls counts how many times one operation was asked for. PRF-11 asserts that
// regaining focus produces one catch-up fetch rather than six.
func (s *Server) Calls(operation string) int {
	n := 0
	for _, r := range s.Requests() {
		if r.Operation == operation {
			n++
		}
	}
	return n
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/graphql", s.graphQL)
	mux.HandleFunc("GET /api/v4/version", s.rest("version"))
	mux.HandleFunc("POST /oauth/authorize_device", s.authorizeDevice)
	mux.HandleFunc("GET /oauth/authorize", s.authorize)
	mux.HandleFunc("POST /oauth/token", s.token)
	mux.HandleFunc("GET /api/v4/user", s.currentUser)
	mux.HandleFunc("GET /api/v4/personal_access_tokens/self", s.tokenSelf)
	mux.HandleFunc("GET /api/v4/projects/{id}/jobs/{job}/trace", s.rest("job.trace"))
	mux.HandleFunc("POST /api/v4/projects/{id}/merge_requests/{iid}/approve", s.rest("mr.approve"))
	mux.HandleFunc("DELETE /api/v4/projects/{id}/merge_requests/{iid}/approve", s.rest("mr.unapprove"))
	mux.HandleFunc("/", s.notFound)
	return mux
}

// graphQLRequest is the wire shape of a GraphQL call.
type graphQLRequest struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

func (s *Server) graphQL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "reading the request body", http.StatusBadRequest)
		return
	}

	var req graphQLRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "the request body is not JSON", http.StatusBadRequest)
		return
	}

	op := req.OperationName
	if op == "" {
		op = operationFromQuery(req.Query)
	}

	s.record(Request{
		Method: r.Method, Path: r.URL.Path, Operation: op,
		Variables: req.Variables, Header: r.Header.Clone(),
	})

	if s.stall(w, r, op) {
		return
	}

	payload, ok := s.body(op)
	if !ok {
		// An unmapped operation is a fixture that has not been written yet.
		// Saying so beats answering an empty object, which reads as a bug in
		// the client.
		s.writeJSON(w, http.StatusOK, map[string]any{
			"errors": []map[string]any{{
				"message": fmt.Sprintf(
					"fakegitlab has no fixture for the operation %q. "+
						"Add testdata/fixtures/graphql/%s.json, or pass "+
						"fakegitlab.WithFixture(%q, body).", op, op, op),
			}},
		})
		return
	}

	if s.failures[op] == PartialResponse {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(withPartialError(payload, op))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (s *Server) rest(operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.record(Request{
			Method: r.Method, Path: r.URL.Path, Operation: operation,
			Header: r.Header.Clone(),
		})

		if s.stall(w, r, operation) {
			return
		}

		if operation == "version" {
			s.writeJSON(w, http.StatusOK, map[string]any{
				"version": s.version, "revision": "fake",
			})
			return
		}

		payload, ok := s.body(operation)
		if !ok {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.record(Request{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone()})
	s.writeJSON(w, http.StatusNotFound, map[string]any{
		"message": fmt.Sprintf("fakegitlab does not serve %s %s", r.Method, r.URL.Path),
	})
}

// stall applies the operation's latency and failure. It reports whether it
// already answered, in which case the caller must not write anything more.
func (s *Server) stall(w http.ResponseWriter, r *http.Request, operation string) bool {
	if d := s.latency[operation]; d > 0 {
		select {
		case <-time.After(d):
		case <-r.Context().Done():
			return true
		}
	}

	f := s.failures[operation]
	switch f {
	case NoFailure, PartialResponse:
		return false
	case Timeout:
		<-r.Context().Done()
		return true
	case RateLimited:
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(s.retryAfter.Seconds())))
		w.Header().Set("RateLimit-Remaining", "0")
		s.writeJSON(w, f.status(), map[string]any{
			"message": "429 Too Many Requests",
		})
		return true
	default:
		s.writeJSON(w, f.status(), map[string]any{
			"message": http.StatusText(f.status()),
		})
		return true
	}
}

// body returns the response for an operation: an explicit override, then the
// next page if the operation paginates, then the embedded fixture.
func (s *Server) body(operation string) ([]byte, bool) {
	if b, ok := s.bodies[operation]; ok {
		return b, true
	}

	if pages, ok := s.pages[operation]; ok && len(pages) > 0 {
		s.mu.Lock()
		i := s.pageAt[operation]
		if i >= len(pages) {
			i = len(pages) - 1
		}
		s.pageAt[operation] = i + 1
		s.mu.Unlock()
		return pages[i], true
	}

	// GraphQL bodies are always JSON. A REST body may be JSON or a raw log,
	// which is what GET …/jobs/:id/trace returns.
	for _, name := range []string{
		"testdata/fixtures/graphql/" + operation + ".json",
		"testdata/fixtures/rest/" + operation + ".json",
		"testdata/fixtures/rest/" + operation + ".txt",
	} {
		if b, err := fs.ReadFile(fixtures, name); err == nil {
			return b, true
		}
	}
	return nil, false
}

func (s *Server) record(r Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// withPartialError adds a top-level errors array beside the data GitLab did
// manage to resolve. A client that reads errors and ignores data, or the
// reverse, is wrong in both directions and this is what catches it.
func withPartialError(payload []byte, operation string) []byte {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(payload, &doc); err != nil {
		return payload
	}
	errs, _ := json.Marshal([]map[string]any{{
		"message": fmt.Sprintf("Field resolution failed for part of %s", operation),
		"path":    []string{operation},
	}})
	doc["errors"] = errs
	out, err := json.Marshal(doc)
	if err != nil {
		return payload
	}
	return out
}

// operationFromQuery reads the operation name out of the document when the
// client did not send one. genqlient always sends one; a hand-written query in
// a test might not.
func operationFromQuery(query string) string {
	for _, line := range strings.Split(query, "\n") {
		line = strings.TrimSpace(line)
		for _, kw := range []string{"query ", "mutation "} {
			if !strings.HasPrefix(line, kw) {
				continue
			}
			name := strings.TrimPrefix(line, kw)
			if i := strings.IndexAny(name, "({ "); i >= 0 {
				name = name[:i]
			}
			return strings.TrimSpace(name)
		}
	}
	return "anonymous"
}
