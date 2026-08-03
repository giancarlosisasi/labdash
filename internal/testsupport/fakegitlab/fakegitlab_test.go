package fakegitlab_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/giancarlosisasi/labdash/internal/testsupport/fakegitlab"
	"github.com/giancarlosisasi/labdash/internal/testsupport/harness"
)

func TestMain(m *testing.M) { harness.Main(m) }

// Prevents: a fake that answers every operation with the same body, which
// would make an integration test pass against a client that asked for the
// wrong thing.
func TestTheFixtureShapeReachesTheClient(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t)

	var doc struct {
		Data struct {
			CurrentUser struct {
				Username string `json:"username"`
			} `json:"currentUser"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(graphQL(t, srv, "CurrentUser"), &doc))
	require.Equal(t, "rlopez", doc.Data.CurrentUser.Username)
}

// Prevents: an unmapped operation answering an empty object, which reads as a
// bug in the client rather than a missing fixture.
func TestAnUnmappedOperationSaysSo(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t)
	body := graphQL(t, srv, "NoSuchOperation")

	require.Contains(t, string(body), "no fixture")
	require.Contains(t, string(body), "NoSuchOperation")
}

// Prevents: latency being a sleep the whole server takes rather than one
// operation. PRF-01 asserts six sections at 3 s each finish in about 4 s, and
// that is only measurable if the fake stalls per operation.
func TestLatencyIsPerOperation(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t, fakegitlab.WithLatency("ReviewRequested", 120*time.Millisecond))

	start := time.Now()
	graphQL(t, srv, "CurrentUser")
	require.Less(t, time.Since(start), 100*time.Millisecond,
		"an operation with no latency configured was delayed anyway")

	start = time.Now()
	graphQL(t, srv, "ReviewRequested")
	require.GreaterOrEqual(t, time.Since(start), 120*time.Millisecond)
}

// Prevents: a 429 without Retry-After. PRF-05 backs off by that header, and a
// fake that omits it lets a client pass while it guesses.
func TestRateLimitedCarriesRetryAfter(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t,
		fakegitlab.WithFailure("ReviewRequested", fakegitlab.RateLimited),
		fakegitlab.WithRetryAfter(7*time.Second))

	resp := post(t, srv, "ReviewRequested")
	defer resp.Body.Close()

	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	require.Equal(t, "7", resp.Header.Get("Retry-After"))
	require.Equal(t, "0", resp.Header.Get("RateLimit-Remaining"))
}

// Prevents: a client that reads GraphQL's errors array and drops the data
// beside it, or the reverse. GitLab half-fails routinely and both halves
// matter.
func TestPartialResponseCarriesDataAndErrors(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t, fakegitlab.WithFailure("CurrentUser", fakegitlab.PartialResponse))

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(graphQL(t, srv, "CurrentUser"), &doc))

	require.Contains(t, doc, "data")
	require.Contains(t, doc, "errors")
}

// Prevents: a "timeout" that answers slowly. PRF-04 needs a section that never
// responds at all, so the fetch budget is what ends the request.
func TestTimeoutNeverAnswers(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t, fakegitlab.WithFailure("CurrentUser", fakegitlab.Timeout))

	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()

	req := newGraphQLRequest(t, ctx, srv, "CurrentUser")
	_, err := http.DefaultClient.Do(req)

	require.Error(t, err, "an operation configured to time out answered anyway")
	require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}

// Prevents: pagination that returns the first page forever, which would make
// PRF-16's 2,000-row walk pass without ever paging.
func TestPagesAdvance(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t, fakegitlab.WithPages("ReviewRequested", [][]byte{
		[]byte(`{"data":{"page":1}}`),
		[]byte(`{"data":{"page":2}}`),
	}))

	require.JSONEq(t, `{"data":{"page":1}}`, string(graphQL(t, srv, "ReviewRequested")))
	require.JSONEq(t, `{"data":{"page":2}}`, string(graphQL(t, srv, "ReviewRequested")))
	require.JSONEq(t, `{"data":{"page":2}}`, string(graphQL(t, srv, "ReviewRequested")),
		"paging past the last page should hold, not wrap")
}

// Prevents: a fake that cannot answer which hosts were contacted. DIA-06.T2
// asserts that exactly two hosts are ever reached, and it reads this record.
func TestEveryRequestIsRecorded(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t)
	graphQL(t, srv, "CurrentUser")
	graphQL(t, srv, "CurrentUser")
	graphQL(t, srv, "ReviewRequested")

	require.Len(t, srv.Requests(), 3)
	require.Equal(t, 2, srv.Calls("CurrentUser"))
	require.Equal(t, 1, srv.Calls("ReviewRequested"))
	require.Equal(t, "/api/graphql", srv.Requests()[0].Path)
}

// Prevents: the version probe being GraphQL. DIA-07 reads GET /version over
// REST, because a self-managed instance too old for our schema still answers it.
func TestVersionIsServedOverREST(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t, fakegitlab.WithVersion("16.11.0"))

	resp, err := http.Get(srv.URL + "/api/v4/version")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "16.11.0")
}

// Prevents: the four REST gaps drifting. research/10-graphql-vs-rest.md §4
// names them, and a route that stops answering is a gap that reopened.
func TestTheFourRESTRoutesAnswer(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t)

	trace, err := http.Get(srv.URL + "/api/v4/projects/812/jobs/88224/trace")
	require.NoError(t, err)
	defer trace.Body.Close()
	require.Equal(t, http.StatusOK, trace.StatusCode)

	body, err := io.ReadAll(trace.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "gitlab-runner")

	approve, err := http.Post(
		srv.URL+"/api/v4/projects/812/merge_requests/2841/approve", "application/json", nil)
	require.NoError(t, err)
	defer approve.Body.Close()
	require.Equal(t, http.StatusOK, approve.StatusCode)
}

// Prevents: an unknown path answering 200, which hides a client asking for the
// wrong endpoint.
func TestAnUnknownPathIsNotFound(t *testing.T) {
	t.Parallel()

	srv := fakegitlab.New(t)

	resp, err := http.Get(srv.URL + "/api/v4/nothing/here")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func newGraphQLRequest(
	t *testing.T, ctx context.Context, srv *fakegitlab.Server, operation string,
) *http.Request {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"operationName": operation,
		"query":         "query " + operation + " { __typename }",
	})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, srv.URL+"/api/graphql", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	return req
}

func post(t *testing.T, srv *fakegitlab.Server, operation string) *http.Response {
	t.Helper()

	resp, err := http.DefaultClient.Do(newGraphQLRequest(t, t.Context(), srv, operation))
	require.NoError(t, err)
	return resp
}

func graphQL(t *testing.T, srv *fakegitlab.Server, operation string) []byte {
	t.Helper()

	resp := post(t, srv, operation)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return body
}
