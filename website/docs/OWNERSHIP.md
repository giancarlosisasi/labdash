# Page ownership

Every page on the site is a promise, because the site is written in the finished tense and
describes the product as it will be. **A page that no change is obliged to make true is how a
finished-tense site turns into fiction**, so every page names the change that owns it.

- **The owner** is the change that first makes the page true end to end — its structure, its
  lede, its links, and every section describing a feature that change delivers.
- **An extender** adds a section the owner could not make true, and does not inherit the
  obligation for the whole page. Until an extender ships, those sections describe a feature
  that does not exist yet.

This table mirrors §6 of `research/18-change-roadmap.md`, which is the planning source. It is
kept here as well because the planning documents live outside this repository and CI has to be
able to read it. `internal/docs` asserts that the two columns below and the `.mdx` files on
disk are the same set: a page with no row fails the build, and a row with no page fails it too.

**Adding a page means adding a row, in the same commit.**

| Page | Owned by | Extended by |
| --- | --- | --- |
| `/` | C14b | — |
| `/guide/` | C14b | — |
| `/guide/install` | C14a | C25, C37a |
| `/guide/first-run` | C03 | C05, C07b, C24b |
| `/guide/read-only` | C03 | — |
| `/guide/self-hosted` | C03 | C33a |
| `/guide/tour` | C06 | C13b, C23b, C29 |
| `/guide/daily-workflow` | C14b | — |
| `/guide/updating` | C24b | C37a |
| `/features/` | C14b | — |
| `/features/home` | C05 | C22a, C31 |
| `/features/browse` | C06 | C22b |
| `/features/filtering` | C07a | C11, C22b, C29 |
| `/features/pinned-views` | C07b | C22b, C31 |
| `/features/review-queue` | C04a | — |
| `/features/merge-requests` | C04b | C10, C17b, C18, C23, C23b, C30a, C30b |
| `/features/why-it-cant-merge` | C09 | — |
| `/features/pipelines` | C12 | C13, C28, C36 |
| `/features/group-pipelines` | C12 | — |
| `/features/staying-fresh` | C08 | C24a |
| `/features/job-logs` | C15a | C15b, C36 |
| `/features/failure-summary` | C16 | — |
| `/features/merge-trains` | C17 | — |
| `/features/todos` | C19 | C32 |
| `/features/local-git` | C21 | C30a |
| `/features/watch-and-notify` | C27 | — |
| `/features/issues` | C34 | — |
| `/features/environments` | C35 | — |
| `/features/multi-instance` | C33a | C33b |
| `/settings/` | C04a | — |
| `/settings/instances` | C33a | C03 |
| `/settings/appearance` | C20 | C26 |
| `/keys/` | C02 | C15b, C17, C17b, C18, C21, C23b, C28, C30a, C32, C35 |
| `/recipes/` | C14b | — |
| `/recipes/review-queue` | C14b | — |
| `/recipes/failed-pipelines` | C14b | — |
| `/recipes/flaky-tests` | C28 | — |
| `/recipes/release-day` | C28 | — |
| `/recipes/scripting` | C31 | — |
| `/recipes/two-gitlabs` | C33a | — |
| `/recipes/corporate-proxy` | C33b | — |
| `/help/` | C14b | — |
| `/help/faq` | C14b | C04b |
| `/help/privacy` | C14b | C24b |
| `/help/security` | C03 | — |
| `/help/troubleshooting` | C14b | C24b |
| `/help/diagnostics` | C24b | — |
| `/help/terminal-support` | C32 | C37a |
| `/about/design` | C20 | C01 |
| `/about/contributing` | C01 | — |
| `/compare/` | C14b | — |
| `/compare/vs-gh-dash` | C37a | — |
| `/compare/vs-glab` | C37a | — |
| `/compare/vs-gitlab-tuis` | C37a | — |

## Generated pages

Two pages are generated from the Go source and must never be hand-edited. CI regenerates both
and fails on a diff.

| Page | Generated from |
| --- | --- |
| `/keys/` | `labdash keys --markdown` |
| `/settings/instances` | the settings structs |
