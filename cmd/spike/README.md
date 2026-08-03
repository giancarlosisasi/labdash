# cmd/spike — GitLab API probe

Validates the assumptions in [`research/10-graphql-vs-rest.md`](../../../research/10-graphql-vs-rest.md)
against a real GitLab instance, before any UI code exists.

Credential discovery lives in [`internal/gitlabauth`](../../internal/gitlabauth/), which is
real code, not spike code — it reproduces glab's own resolution algorithm.

## Run it

```powershell
# Check credential discovery only. Zero API calls. Start here on a new machine.
go run ./cmd/spike -auth-only

# See exactly what it will send. Also zero network.
go run ./cmd/spike -dry-run

# Real run. Picks one of your groups automatically.
go run ./cmd/spike

# Against a specific group, on a specific host.
go run ./cmd/spike -group my-org/platform -host gitlab.example.com
```

## It never sees your token

The program resolves the token **inside its own process** and prints only where it came
from and how many characters it has. `Credentials.String()` redacts the value, so even an
accidental `%v` in a future log line cannot leak it.

Resolution order, matching glab's own (read from glab's source, not its docs):

1. `$GITLAB_TOKEN`, `$GITLAB_ACCESS_TOKEN`, `$OAUTH_TOKEN` — **default host only**, so a
   global variable cannot leak one instance's token to another
2. `$GLAB_CONFIG_DIR/config.yml` — short-circuits everything else
3. `~/.config/glab-cli/config.yml` — legacy path, checked first on **every** OS
4. the platform XDG config home:

   | OS | Path |
   | --- | --- |
   | Windows | `%LOCALAPPDATA%\glab-cli\config.yml` |
   | macOS | `~/Library/Application Support/glab-cli/config.yml` |
   | Linux | `~/.config/glab-cli/config.yml` |

5. system-wide XDG config dirs
6. the OS keyring: `glab:<host>:token`, then legacy `glab:<host>`

Step 6 matters more than it looks: modern glab stores the token in the keyring **by
default**, so on a freshly onboarded machine the config file's `token:` is empty.

`-auth-only` prints this whole list with a `*` beside the files that exist.

If you would rather use an environment variable, set it without echoing it:

```powershell
# PowerShell
$env:GITLAB_TOKEN = (Read-Host 'GitLab token' -MaskInput)
```
```bash
# bash / zsh
read -rs GITLAB_TOKEN && export GITLAB_TOKEN
```

A `read_api` token is enough — every probe is read-only.

## OAuth vs personal access token

`glab auth login` against gitlab.com mints an **OAuth** token by default, and GitLab expires
those after **two hours**. glab refreshes its own copy when you run a glab command; a
dashboard left open does not.

The spike detects this and prints a note. It deliberately does **not** refresh the token:
GitLab rotates the refresh token on use, which would invalidate glab's copy and break your
CLI. For anything long-running, create a PAT and set `GITLAB_TOKEN`.

## It will not get you rate limited

GitLab.com allows 2,000 authenticated API requests per minute. Measured on a full run:
**7 requests, `RateLimit-Remaining: 1993`.** The program also:

- refuses to make more than **12** requests, ever (`maxRequests`);
- waits **400 ms** between requests, so it cannot exceed ~150/min even if it looped;
- times out any single request after **25 s**, and the whole run after **3 min**;
- treats HTTP **429 as fatal** — it stops and prints `Retry-After` rather than retrying;
- prints the `RateLimit-*` headers so you can see the real headroom.

## Pagination

Every list is capped at **50** records, in GraphQL (`first: 50`) and REST (`per_page=50`).
Override with `-page-size`.

`-nested-page-size` controls connections nested inside another connection — only probe 5.
Measured at 50×50 it scores 52/250 complexity, so it is not actually at risk; the flag stays
for self-managed instances that may score differently.

## What each probe proved

Measured 2026-08-01 against gitlab.com. Full analysis in
[`research/10-graphql-vs-rest.md` §2](../../../research/10-graphql-vs-rest.md).

| # | Probe | Complexity | Time | Proved |
| --- | --- | --- | --- | --- |
| 1 | `currentUser` | 6/250 | 0.2 s | The token authenticates. |
| 2 | `currentUser.groups` | 8/250 | 0.4 s | Picks a group for probes 3 and 5. |
| 3 | **Spike A** — `group.mergeRequests` | 59/250 | **6.6 s** | Open MRs across a group *and its subgroups*, with approvals, merge blockers, and head-pipeline status, in **one round trip**. 1,447 open MRs found. |
| 4 | **Spike A2** — `currentUser.reviewRequestedMergeRequests` | 37/250 | 4.6 s | "Needs my review" across every visible project, no group enumeration. GitHub has no single-call equivalent. |
| 5 | `group.projects.nodes.pipelines` | 52/250 | 4.4 s | `Group.pipelines` does not exist; the nested workaround returns 945 pipelines from 50 projects and fits easily. |
| 6 | `GET /version` | n/a | 0.2 s | REST reachable. gitlab.com runs 19.3.0-pre. |
| 7 | `GET /merge_requests?per_page=50` | n/a | 2.0 s | REST pagination headers behave as expected. |

**The headline: complexity is a non-issue, latency is the constraint.** GitLab scores
complexity by field count, not page size. Design around the 4–7 s, not the 250 points.
