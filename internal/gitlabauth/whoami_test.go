package gitlabauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIdentityGreeting(t *testing.T) {
	tests := []struct {
		name string
		id   Identity
		want string
	}{
		{
			name: "name and handle",
			id:   Identity{Name: "Giancarlos Isasi", Username: "giancarlos1"},
			want: "Giancarlos Isasi (@giancarlos1)",
		},
		{name: "handle only", id: Identity{Username: "giancarlos1"}, want: "@giancarlos1"},
		{name: "name only", id: Identity{Name: "Giancarlos Isasi"}, want: "Giancarlos Isasi"},
		{name: "nothing", id: Identity{}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.Greeting(); got != tt.want {
				t.Errorf("Greeting() = %q, want %q", got, tt.want)
			}
		})
	}
}

// credsFor points a Credentials at a test server.
func credsFor(srv *httptest.Server) Credentials {
	host := strings.TrimPrefix(srv.URL, "http://")
	return Credentials{
		Host:        host,
		APIHost:     host,
		APIProtocol: "http",
		Token:       "test-token",
	}
}

func TestWhoAmIReadsIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v4/user" {
			t.Errorf("path = %q, want /api/v4/user", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "username":"giancarlos1",
            "name":"Giancarlos Isasi",
            "email":"real@example.com",
            "avatar_url":"https://gitlab.com/uploads/avatar.png",
            "web_url":"https://gitlab.com/giancarlos1"
        }`))
	}))
	defer srv.Close()

	id, err := WhoAmI(context.Background(), credsFor(srv), srv.Client())
	if err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
	if id.Username != "giancarlos1" || id.Name != "Giancarlos Isasi" {
		t.Errorf("identity = %+v", id)
	}
	// REST returns the real address, which is the whole reason this is not
	// GraphQL: currentUser.email has been deprecated since GitLab 13.7 and
	// only ever returned the (usually empty) public email.
	if id.Email != "real@example.com" {
		t.Errorf("Email = %q, want the real address from REST", id.Email)
	}
}

func TestWhoAmIHonoursSubfolder(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"u","name":"N"}`))
	}))
	defer srv.Close()

	creds := credsFor(srv)
	creds.Subfolder = "gitlab"

	if _, err := WhoAmI(context.Background(), creds, srv.Client()); err != nil {
		t.Fatalf("WhoAmI: %v", err)
	}
	if gotPath != "/gitlab/api/v4/user" {
		t.Errorf("path = %q, want /gitlab/api/v4/user — self-hosted subfolder was dropped", gotPath)
	}
}

func TestWhoAmIErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantSubstr string
	}{
		{name: "rejected", status: http.StatusUnauthorized, wantSubstr: "rejected"},
		{name: "missing scope", status: http.StatusForbidden, wantSubstr: "read_user"},
		{name: "server error", status: http.StatusInternalServerError, wantSubstr: "HTTP 500"},
		{
			name:       "empty user",
			status:     http.StatusOK,
			body:       `{}`,
			wantSubstr: "not authenticating",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				if tt.body != "" {
					_, _ = w.Write([]byte(tt.body))
				}
			}))
			defer srv.Close()

			_, err := WhoAmI(context.Background(), credsFor(srv), srv.Client())
			if err == nil {
				t.Fatalf("WhoAmI succeeded against HTTP %d", tt.status)
			}
			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Errorf("error %q does not mention %q", err, tt.wantSubstr)
			}
		})
	}
}

func TestScopesIncludeReadUser(t *testing.T) {
	for name, scopes := range map[string][]string{
		"write": writeScopes,
		"read":  readScopes,
	} {
		found := false
		for _, s := range scopes {
			if s == "read_user" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s scopes %v are missing read_user, so /user cannot return the email",
				name, scopes)
		}
	}

	// The read-only mode must never ask for write access.
	for _, s := range readScopes {
		if s == "api" {
			t.Error("read-only scopes must not include api")
		}
	}
}
