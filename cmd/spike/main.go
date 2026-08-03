// Command spike probes the GitLab GraphQL and REST APIs to validate the
// architectural assumptions recorded in research/10-graphql-vs-rest.md.
//
// It answers four questions with real data:
//
//  1. Does a group's merge requests plus head-pipeline status come back in one
//     round trip, and what does it cost against the 250-point complexity limit?
//  2. Does currentUser.reviewRequestedMergeRequests give "needs my review"
//     across every project without enumerating groups?
//  3. Group.pipelines does not exist. Does the nested
//     group.projects.nodes.pipelines workaround fit inside the complexity budget?
//  4. Is the REST fallback reachable, and does it paginate the way we expect?
//
// It never prints the token. It reports only where the token came from and how
// many characters it has.
//
// Usage:
//
//	go run ./cmd/spike                      # auto-picks one of your groups
//	go run ./cmd/spike -group my/group
//	go run ./cmd/spike -dry-run             # print the queries, make no calls
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/giancarlosisasi/labdash/internal/gitlabauth"
)

// Every list we ask for is capped at this many records, in GraphQL and in REST
// alike. See research/10-graphql-vs-rest.md.
const defaultPageSize = 50

// Politeness budget. GitLab.com allows 2,000 authenticated API requests per
// minute; minRequestSpacing holds us to roughly 150/min, and maxRequests stops
// a bug from turning into a loop. Neither limit should ever be reached by a
// healthy run, which makes tripping one a signal rather than an inconvenience.
const (
	maxRequests       = 12
	minRequestSpacing = 400 * time.Millisecond
	httpTimeout       = 25 * time.Second
)

func main() {
	cfg := parseFlags()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := run(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "\n  spike failed: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	host           string
	group          string
	pageSize       int
	nestedPageSize int
	dryRun         bool
	authOnly       bool
}

func parseFlags() config {
	cfg := config{}

	flag.StringVar(&cfg.host, "host", "",
		"GitLab host; empty means $GITLAB_HOST, then gitlab.com")
	flag.StringVar(&cfg.group, "group", "", "group full path; empty means auto-pick one of yours")
	flag.IntVar(&cfg.pageSize, "page-size", defaultPageSize, "records per page")
	flag.IntVar(
		&cfg.nestedPageSize,
		"nested-page-size",
		defaultPageSize,
		"records per page for connections nested inside another connection; lower this if probe 5 blows the complexity budget",
	)
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "print the queries and exit without calling the API")
	flag.BoolVar(&cfg.authOnly, "auth-only", false,
		"resolve credentials, print where they came from, and exit without calling the API")
	flag.Parse()

	return cfg
}

func run(ctx context.Context, cfg config) error {
	fmt.Printf("labdash spike  pageSize=%d  nestedPageSize=%d\n",
		cfg.pageSize, cfg.nestedPageSize)
	fmt.Println(strings.Repeat("=", 78))

	if cfg.dryRun {
		printQueries()
		return nil
	}

	// An empty Host lets gitlabauth read $GITLAB_HOST and apply the token
	// environment variables; naming a host explicitly deliberately does not.
	creds, err := gitlabauth.Resolve(gitlabauth.Options{Host: cfg.host})
	if err != nil {
		return err
	}

	// Renew if this is our own OAuth credential and it is near expiry. A token
	// supplied through an environment variable is returned untouched: we did
	// not mint it, so we hold no refresh token for it.
	if creds.NeedsRefresh() {
		fmt.Println("\n  renewing our OAuth token before starting...")
		if creds, err = gitlabauth.Refresh(ctx, creds, gitlabauth.LoginOptions{}); err != nil {
			return err
		}
	}

	fmt.Printf("\ntoken     : %s\n", creds.Describe())
	fmt.Printf("graphql   : %s\n", creds.GraphQLEndpoint())
	fmt.Printf("rest      : %s\n", creds.RESTEndpoint())
	if advice := creds.Advisory(); advice != "" {
		fmt.Printf("\n  note: %s\n", advice)
	}

	if cfg.authOnly {
		printAuthDetail(creds, cfg)
		return nil
	}

	c := newClient(creds)

	report := []probeResult{}
	record := func(r probeResult, err error) error {
		report = append(report, r)
		return err
	}

	if err := record(probeViewer(ctx, c)); err != nil {
		return err
	}

	group := cfg.group
	if group == "" {
		res, picked, err := probePickGroup(ctx, c, cfg.pageSize)
		report = append(report, res)
		if err != nil {
			return err
		}
		group = picked
	}

	if group == "" {
		fmt.Println("\n  no group available — skipping probes 3 and 5")
		fmt.Println("  pass -group <full/path> to run them against a specific group")
	} else {
		if err := record(probeGroupMergeRequests(ctx, c, group, cfg.pageSize)); err != nil {
			return err
		}
	}

	if err := record(probeReviewRequested(ctx, c, cfg.pageSize)); err != nil {
		return err
	}

	if group != "" {
		if err := record(probeGroupPipelines(ctx, c, group, cfg.pageSize, cfg.nestedPageSize)); err != nil {
			return err
		}
	}

	if err := record(probeRESTVersion(ctx, c)); err != nil {
		return err
	}
	if err := record(probeRESTMergeRequests(ctx, c, cfg.pageSize)); err != nil {
		return err
	}

	printSummary(report, c)

	return nil
}

// ---------------------------------------------------------------------------
// HTTP client
// ---------------------------------------------------------------------------

// client is a deliberately slow, self-limiting GitLab client. It spaces
// requests apart, refuses to exceed a fixed budget, and treats a 429 as fatal
// rather than retrying into a ban.
type client struct {
	creds gitlabauth.Credentials
	http  *http.Client

	count int
	last  time.Time
}

func newClient(creds gitlabauth.Credentials) *client {
	return &client{
		creds: creds,
		http:  &http.Client{Timeout: httpTimeout},
	}
}

var errBudgetExhausted = errors.New("request budget exhausted — refusing to make more calls")

func (c *client) do(ctx context.Context, req *http.Request) ([]byte, *http.Response, error) {
	if c.count >= maxRequests {
		return nil, nil, errBudgetExhausted
	}

	if wait := minRequestSpacing - time.Since(c.last); wait > 0 && !c.last.IsZero() {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}

	req.Header.Set("Authorization", "Bearer "+c.creds.Token)
	req.Header.Set("User-Agent", "labdash-spike/0.1")
	for _, h := range c.creds.CustomHeaders {
		req.Header.Set(h.Name, h.Resolve())
	}

	c.count++
	c.last = time.Now()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request to %s: %w", req.URL.Host, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return body, resp, fmt.Errorf(
			"rate limited (429), Retry-After=%q — stopping so we stay a good citizen",
			resp.Header.Get("Retry-After"))
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return body, resp, fmt.Errorf(
			"HTTP %d — the token was rejected or lacks the scope for this call", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return body, resp, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	return body, resp, nil
}

// ---------------------------------------------------------------------------
// GraphQL
// ---------------------------------------------------------------------------

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlError struct {
	Message string `json:"message"`
}

type gqlEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

type complexity struct {
	Score int `json:"score"`
	Limit int `json:"limit"`
}

func (c *client) graphql(
	ctx context.Context,
	query string,
	vars map[string]any,
	out any,
) (complexity, error) {
	payload, err := json.Marshal(gqlRequest{Query: query, Variables: vars})
	if err != nil {
		return complexity{}, fmt.Errorf("encoding GraphQL request: %w", err)
	}

	url := c.creds.GraphQLEndpoint()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return complexity{}, fmt.Errorf("building GraphQL request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	body, _, err := c.do(ctx, req)
	if err != nil {
		return complexity{}, err
	}

	env := gqlEnvelope{}
	if err := json.Unmarshal(body, &env); err != nil {
		return complexity{}, fmt.Errorf("decoding GraphQL envelope: %w", err)
	}

	// The complexity block is read before the error check on purpose: when a
	// query is rejected *for* being too complex, this is the number we came for.
	cx := complexity{}
	probe := struct {
		QueryComplexity complexity `json:"queryComplexity"`
	}{}
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &probe)
		cx = probe.QueryComplexity
	}

	if len(env.Errors) > 0 {
		msgs := make([]string, 0, len(env.Errors))
		for _, e := range env.Errors {
			msgs = append(msgs, e.Message)
		}
		return cx, fmt.Errorf("GraphQL: %s", strings.Join(msgs, "; "))
	}

	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return cx, fmt.Errorf("decoding GraphQL data: %w", err)
		}
	}

	return cx, nil
}

// ---------------------------------------------------------------------------
// Probes
// ---------------------------------------------------------------------------

type probeResult struct {
	name     string
	ok       bool
	cx       complexity
	elapsed  time.Duration
	detail   string
	skipHint string
}

const viewerQuery = `query Viewer {
  queryComplexity { score limit }
  currentUser { username name }
}`

func probeViewer(ctx context.Context, c *client) (probeResult, error) {
	r := probeResult{name: "1. connectivity — currentUser"}
	start := time.Now()

	out := struct {
		CurrentUser *struct {
			Username string `json:"username"`
			Name     string `json:"name"`
		} `json:"currentUser"`
	}{}

	cx, err := c.graphql(ctx, viewerQuery, nil, &out)
	r.cx, r.elapsed = cx, time.Since(start)
	if err != nil {
		return r, err
	}
	if out.CurrentUser == nil {
		return r, errors.New("currentUser was null — the token is not authenticating")
	}

	r.ok = true
	r.detail = fmt.Sprintf("authenticated as %s", out.CurrentUser.Username)
	report(r)

	return r, nil
}

const myGroupsQuery = `query MyGroups($n: Int!) {
  queryComplexity { score limit }
  currentUser {
    groups(first: $n) { nodes { fullPath } }
  }
}`

func probePickGroup(ctx context.Context, c *client, n int) (probeResult, string, error) {
	r := probeResult{name: "2. pick a group — currentUser.groups"}
	start := time.Now()

	out := struct {
		CurrentUser *struct {
			Groups struct {
				Nodes []struct {
					FullPath string `json:"fullPath"`
				} `json:"nodes"`
			} `json:"groups"`
		} `json:"currentUser"`
	}{}

	cx, err := c.graphql(ctx, myGroupsQuery, map[string]any{"n": n}, &out)
	r.cx, r.elapsed = cx, time.Since(start)
	if err != nil {
		return r, "", err
	}

	paths := []string{}
	if out.CurrentUser != nil {
		for _, g := range out.CurrentUser.Groups.Nodes {
			paths = append(paths, g.FullPath)
		}
	}

	r.ok = true
	if len(paths) == 0 {
		r.detail = "you belong to no groups on this host"
		report(r)
		return r, "", nil
	}

	r.detail = fmt.Sprintf("%d group(s); using %q", len(paths), paths[0])
	report(r)

	return r, paths[0], nil
}

// groupMergeRequestsQuery is Spike A: every open MR in a group and its
// subgroups, with approvals, merge blockers, and head-pipeline status, in one
// round trip. Doc 03 calls this the product thesis.
const groupMergeRequestsQuery = `query GroupMRs($path: ID!, $n: Int!) {
  queryComplexity { score limit }
  group(fullPath: $path) {
    mergeRequests(
      state: opened
      includeSubgroups: true
      includeArchived: false
      sort: UPDATED_DESC
      first: $n
    ) {
      count
      pageInfo { hasNextPage endCursor }
      nodes {
        iid
        title
        webUrl
        draft
        conflicts
        updatedAt
        sourceBranch
        targetBranch
        approvalsRequired
        approvalsLeft
        detailedMergeStatus
        userNotesCount
        author { username }
        project { fullPath }
        headPipeline { status detailedStatus { text } }
        labels(first: 5) { nodes { title } }
      }
    }
  }
}`

func probeGroupMergeRequests(ctx context.Context, c *client, group string, n int) (probeResult, error) {
	r := probeResult{name: "3. SPIKE A — group MRs + head pipeline, one round trip"}
	start := time.Now()

	out := struct {
		Group *struct {
			MergeRequests struct {
				Count    int `json:"count"`
				PageInfo struct {
					HasNextPage bool `json:"hasNextPage"`
				} `json:"pageInfo"`
				Nodes []struct {
					IID                 string `json:"iid"`
					Title               string `json:"title"`
					Draft               bool   `json:"draft"`
					ApprovalsLeft       *int   `json:"approvalsLeft"`
					DetailedMergeStatus string `json:"detailedMergeStatus"`
					Author              *struct {
						Username string `json:"username"`
					} `json:"author"`
					Project *struct {
						FullPath string `json:"fullPath"`
					} `json:"project"`
					HeadPipeline *struct {
						Status string `json:"status"`
					} `json:"headPipeline"`
				} `json:"nodes"`
			} `json:"mergeRequests"`
		} `json:"group"`
	}{}

	cx, err := c.graphql(
		ctx,
		groupMergeRequestsQuery,
		map[string]any{"path": group, "n": n},
		&out,
	)
	r.cx, r.elapsed = cx, time.Since(start)
	if err != nil {
		return r, err
	}
	if out.Group == nil {
		return r, fmt.Errorf("group %q not found or not visible to this token", group)
	}

	mrs := out.Group.MergeRequests
	r.ok = true
	r.detail = fmt.Sprintf("%d open MR(s) in %s, %d returned, hasNextPage=%t",
		mrs.Count, group, len(mrs.Nodes), mrs.PageInfo.HasNextPage)
	report(r)

	for i, mr := range mrs.Nodes {
		if i >= 5 {
			fmt.Printf("      ... and %d more\n", len(mrs.Nodes)-5)
			break
		}
		fmt.Printf("      %-28s !%-5s %-9s %-22s %s\n",
			truncate(projectPath(mr.Project), 28),
			mr.IID,
			pipelineLabel(mr.HeadPipeline),
			mr.DetailedMergeStatus,
			truncate(mr.Title, 40))
	}

	return r, nil
}

// reviewRequestedQuery is Spike A2: "needs my review" across every project the
// user can see, with no group enumeration. GitHub has no single-call
// equivalent — this is the README demo.
const reviewRequestedQuery = `query NeedsMyReview($n: Int!) {
  queryComplexity { score limit }
  currentUser {
    reviewRequestedMergeRequests(state: opened, sort: UPDATED_DESC, first: $n) {
      count
      pageInfo { hasNextPage endCursor }
      nodes {
        iid
        title
        webUrl
        updatedAt
        draft
        approvalsLeft
        detailedMergeStatus
        author { username }
        project { fullPath }
        headPipeline { status }
      }
    }
  }
}`

func probeReviewRequested(ctx context.Context, c *client, n int) (probeResult, error) {
	r := probeResult{name: "4. SPIKE A2 — needs my review, instance-wide"}
	start := time.Now()

	out := struct {
		CurrentUser *struct {
			ReviewRequestedMergeRequests struct {
				Count int `json:"count"`
				Nodes []struct {
					IID     string `json:"iid"`
					Title   string `json:"title"`
					Project *struct {
						FullPath string `json:"fullPath"`
					} `json:"project"`
					HeadPipeline *struct {
						Status string `json:"status"`
					} `json:"headPipeline"`
				} `json:"nodes"`
			} `json:"reviewRequestedMergeRequests"`
		} `json:"currentUser"`
	}{}

	cx, err := c.graphql(ctx, reviewRequestedQuery, map[string]any{"n": n}, &out)
	r.cx, r.elapsed = cx, time.Since(start)
	if err != nil {
		return r, err
	}

	count := 0
	if out.CurrentUser != nil {
		count = out.CurrentUser.ReviewRequestedMergeRequests.Count
	}

	r.ok = true
	r.detail = fmt.Sprintf("%d MR(s) awaiting your review across all projects", count)
	report(r)

	if out.CurrentUser != nil {
		for i, mr := range out.CurrentUser.ReviewRequestedMergeRequests.Nodes {
			if i >= 5 {
				break
			}
			fmt.Printf("      %-28s !%-5s %-9s %s\n",
				truncate(projectPath(mr.Project), 28),
				mr.IID,
				pipelineLabel(mr.HeadPipeline),
				truncate(mr.Title, 40))
		}
	}

	return r, nil
}

// groupPipelinesQuery is the workaround for the fact that Group.pipelines does
// not exist. It is the riskiest query in the dashboard: two nested connections
// multiply, so this is the one that decides whether group-wide pipeline
// sections are viable as a single call.
const groupPipelinesQuery = `query GroupPipelines($path: ID!, $projects: Int!, $pipelines: Int!) {
  queryComplexity { score limit }
  group(fullPath: $path) {
    projects(includeSubgroups: true, includeArchived: false, first: $projects) {
      count
      pageInfo { hasNextPage endCursor }
      nodes {
        fullPath
        pipelines(first: $pipelines) {
          nodes {
            iid
            status
            ref
            duration
            finishedAt
            detailedStatus { text }
          }
        }
      }
    }
  }
}`

func probeGroupPipelines(ctx context.Context, c *client, group string, projects, pipelines int) (probeResult, error) {
	r := probeResult{
		name: fmt.Sprintf("5. group pipelines, nested %dx%d (Group.pipelines does not exist)",
			projects, pipelines),
	}
	start := time.Now()

	out := struct {
		Group *struct {
			Projects struct {
				Count int `json:"count"`
				Nodes []struct {
					FullPath  string `json:"fullPath"`
					Pipelines struct {
						Nodes []struct {
							IID    string `json:"iid"`
							Status string `json:"status"`
							Ref    string `json:"ref"`
						} `json:"nodes"`
					} `json:"pipelines"`
				} `json:"nodes"`
			} `json:"projects"`
		} `json:"group"`
	}{}

	cx, err := c.graphql(
		ctx,
		groupPipelinesQuery,
		map[string]any{"path": group, "projects": projects, "pipelines": pipelines},
		&out,
	)
	r.cx, r.elapsed = cx, time.Since(start)
	if err != nil {
		r.skipHint = "retry with a smaller -nested-page-size; this is the finding, not a failure"
		report(r)
		fmt.Printf("      %v\n", err)
		// A rejection here is a valid experimental result, so the run continues.
		return r, nil
	}
	if out.Group == nil {
		return r, fmt.Errorf("group %q not found or not visible to this token", group)
	}

	total := 0
	for _, p := range out.Group.Projects.Nodes {
		total += len(p.Pipelines.Nodes)
	}

	r.ok = true
	r.detail = fmt.Sprintf("%d project(s) scanned, %d pipeline(s) returned",
		len(out.Group.Projects.Nodes), total)
	report(r)

	shown := 0
	for _, p := range out.Group.Projects.Nodes {
		for _, pl := range p.Pipelines.Nodes {
			if shown >= 5 {
				break
			}
			fmt.Printf("      %-28s #%-6s %-10s %s\n",
				truncate(p.FullPath, 28), pl.IID, pl.Status, truncate(pl.Ref, 24))
			shown++
		}
	}

	return r, nil
}

func probeRESTVersion(ctx context.Context, c *client) (probeResult, error) {
	r := probeResult{name: "6. REST reachable — GET /version"}
	start := time.Now()

	url := c.creds.RESTEndpoint() + "/version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return r, fmt.Errorf("building version request: %w", err)
	}

	body, _, err := c.do(ctx, req)
	r.elapsed = time.Since(start)
	if err != nil {
		return r, err
	}

	out := struct {
		Version  string `json:"version"`
		Revision string `json:"revision"`
	}{}
	if err := json.Unmarshal(body, &out); err != nil {
		return r, fmt.Errorf("decoding version: %w", err)
	}

	r.ok = true
	r.detail = fmt.Sprintf("GitLab %s (%s)", out.Version, out.Revision)
	report(r)

	return r, nil
}

func probeRESTMergeRequests(ctx context.Context, c *client, perPage int) (probeResult, error) {
	r := probeResult{name: fmt.Sprintf("7. REST pagination — per_page=%d", perPage)}
	start := time.Now()

	url := fmt.Sprintf(
		"%s/merge_requests?scope=assigned_to_me&state=opened&per_page=%d&page=1",
		c.creds.RESTEndpoint(), perPage)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return r, fmt.Errorf("building merge_requests request: %w", err)
	}

	body, resp, err := c.do(ctx, req)
	r.elapsed = time.Since(start)
	if err != nil {
		return r, err
	}

	rows := []struct {
		IID int `json:"iid"`
	}{}
	if err := json.Unmarshal(body, &rows); err != nil {
		return r, fmt.Errorf("decoding merge_requests: %w", err)
	}

	r.ok = true
	r.detail = fmt.Sprintf("%d row(s); X-Per-Page=%q X-Next-Page=%q X-Total=%q",
		len(rows),
		resp.Header.Get("X-Per-Page"),
		resp.Header.Get("X-Next-Page"),
		resp.Header.Get("X-Total"))
	report(r)

	if limit := resp.Header.Get("RateLimit-Limit"); limit != "" {
		fmt.Printf("      rate limit: %s remaining of %s, resets at %s\n",
			resp.Header.Get("RateLimit-Remaining"), limit, resp.Header.Get("RateLimit-Reset"))
	}

	return r, nil
}

// ---------------------------------------------------------------------------
// Output
// ---------------------------------------------------------------------------

func report(r probeResult) {
	mark := "FAIL"
	if r.ok {
		mark = " ok "
	}

	fmt.Printf("\n[%s] %s\n", mark, r.name)
	if r.cx.Limit > 0 {
		fmt.Printf("      complexity: %d / %d\n", r.cx.Score, r.cx.Limit)
	}
	fmt.Printf("      elapsed   : %s\n", r.elapsed.Round(time.Millisecond))
	if r.detail != "" {
		fmt.Printf("      result    : %s\n", r.detail)
	}
	if r.skipHint != "" {
		fmt.Printf("      hint      : %s\n", r.skipHint)
	}
}

func printSummary(report []probeResult, c *client) {
	fmt.Println("\n" + strings.Repeat("=", 78))
	fmt.Printf("SUMMARY — %d API request(s) of a %d budget\n\n", c.count, maxRequests)
	fmt.Printf("  %-58s %-12s %s\n", "probe", "complexity", "result")
	fmt.Printf("  %s\n", strings.Repeat("-", 74))

	for _, r := range report {
		cx := "n/a"
		if r.cx.Limit > 0 {
			cx = fmt.Sprintf("%d / %d", r.cx.Score, r.cx.Limit)
		}
		mark := "FAIL"
		if r.ok {
			mark = "ok"
		}
		fmt.Printf("  %-58s %-12s %s\n", truncate(r.name, 58), cx, mark)
	}

	fmt.Println("\n  Paste this table into research/10-graphql-vs-rest.md §2.")
}

// printAuthDetail shows how credentials were resolved without making any API
// call. It is the fastest way to check that discovery works on a new OS.
func printAuthDetail(creds gitlabauth.Credentials, cfg config) {
	fmt.Printf("\nresolved credentials (no API calls made)\n")
	fmt.Printf("  host           : %s\n", creds.Host)
	fmt.Printf("  api host       : %s\n", creds.APIHost)
	fmt.Printf("  api protocol   : %s\n", creds.APIProtocol)
	fmt.Printf("  subfolder      : %q\n", creds.Subfolder)
	fmt.Printf("  token source   : %s\n", creds.Source)
	fmt.Printf("  token length   : %d characters (value never printed)\n", len(creds.Token))
	fmt.Printf("  oauth          : %t\n", creds.IsOAuth2)
	fmt.Printf("  ca cert        : %q\n", creds.CACert)
	fmt.Printf("  client cert    : %q\n", creds.ClientCert)
	fmt.Printf("  skip tls verify: %t\n", creds.SkipTLSVerify)
	fmt.Printf("  proxy          : %q\n", creds.Proxy)
	fmt.Printf("  custom headers : %d\n", len(creds.CustomHeaders))

	fmt.Printf("  managed        : %t (ours, renewable)\n", creds.Managed)

	path := gitlabauth.SettingsPath()
	state := "missing"
	if _, err := os.Stat(path); err == nil {
		state = "exists"
	}
	fmt.Printf("\n  config         : %s (%s)\n", path, state)

	if cfg.host == "" {
		fmt.Printf("\nGeneric token environment variables apply to the default host only.\n")
	} else {
		fmt.Printf("\nHost was named explicitly, so generic token environment variables were\n" +
			"skipped on purpose — one instance's token must not leak to another.\n")
	}
}

func printQueries() {
	queries := []struct {
		name string
		body string
	}{
		{"1. connectivity", viewerQuery},
		{"2. pick a group", myGroupsQuery},
		{"3. SPIKE A — group MRs", groupMergeRequestsQuery},
		{"4. SPIKE A2 — needs my review", reviewRequestedQuery},
		{"5. group pipelines (nested)", groupPipelinesQuery},
	}

	for _, q := range queries {
		fmt.Printf("\n--- %s ---\n%s\n", q.name, q.body)
	}

	fmt.Printf("\n--- 6. REST ---\nGET /api/v4/version\nGET /api/v4/merge_requests"+
		"?scope=assigned_to_me&state=opened&per_page=%d&page=1\n", defaultPageSize)
	fmt.Println("\nNo network calls were made.")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

// projectPath and pipelineLabel take the anonymous struct types used in the
// probe response shapes, so both probes can share them.
func projectPath(p *struct {
	FullPath string `json:"fullPath"`
}) string {
	if p == nil {
		return "—"
	}
	return p.FullPath
}

func pipelineLabel(p *struct {
	Status string `json:"status"`
}) string {
	if p == nil {
		return "—"
	}
	return p.Status
}
