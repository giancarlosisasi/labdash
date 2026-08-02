# The labdash docs voice

The rules every page on this site follows. Read this before writing or editing any
`.mdx` file under `docs/`.

---

## 0. Two rules that override everything else

### 0.1 The docs describe a finished product

This site is written **as if labdash is complete and shipping**. It is
documentation-driven development: the site is the specification, and the product is
built to match it.

So nothing on this site may mention:

- what is built today, what is not built yet, what is coming
- milestones, versions-it-lands-in, roadmaps, "planned", "experimental", "beta"
- "we are building", "this will", "once this ships", "for now"
- the project's own history, or decisions made during planning

Write in the **present tense, indicative mood**. `labdash shows the failed jobs.` Not
`labdash will show` and not `labdash is designed to show`.

There is no `status:` or `since:` frontmatter. There is no `<Status>` component. There
is no roadmap page. If you feel the urge to hedge, delete the hedge.

### 0.2 The reader is not being sold to

Somebody reading these pages has already decided to look. They want to know what the
thing does and which key does it. They do not want a diagnosis of their working life.

Never write:

- a "The problem" section, in any wording
- a description of the reader's frustration, workflow, or browser tabs
- "you already know", "you have felt this", "sound familiar"
- a comparison used as a pitch rather than as a fact
- a manifesto: "what we deliberately are not", "saying no in public", "the whole point"

State what the feature is. Then state what it does. Then state how to use it.

---

## 1. Banned constructions

These are the tells. Each one appears in the copy this rewrite replaced.

| Do not write | Why | Write instead |
| --- | --- | --- |
| `A — B` (em dash as a beat) | The single strongest AI tell. Two per page, maximum, and never in a heading or a first sentence. | A full stop. Or a comma. Or a colon. |
| `Not X, but Y` / `X, not Y` | Rhetorical inversion. Defines by negation. | Say Y. |
| `It is not that X. It is that Y.` | Same, longer. | Say Y. |
| `X does what Y used to.` | Metaphor as headline. | Name X. |
| Headline that hides its subject: *Why it cannot merge, in words* | The reader cannot scan for it. | *Merge blockers* |
| `That is the entire product surface.` | Self-congratulation. | Delete. |
| `with nothing rounded up`, `measured on the real API`, `verified, not claimed` | Precision theatre. Defends against an accusation nobody made. | Give the number, or drop it. |
| Fragment for emphasis. `Every time.` `And it works.` | Ad-copy rhythm. | Join it to the sentence before. |
| Rule of three: `no file, no wizard, no schema` | Fine once on a page. Three times is a tic. | Use one, or a list. |
| Rhetorical question as a heading | | A noun phrase. |
| `simply`, `just`, `merely`, `of course`, `obviously` | | Delete. |
| `powerful`, `seamless`, `blazing`, `delightful`, `first-class` | | Say the concrete thing. |
| `deliberately`, `intentionally`, `on purpose` | Defending a decision. The reader did not ask. | Delete. |
| Starting a sentence with `And` or `But` for rhythm | Once a page at most. | |
| `Think of it as…` | | Describe it directly. |

**Em dash budget: two per page.** Count them before you save. An em dash is correct for
a genuine parenthetical aside. It is not correct as a drum beat before a punchline.

---

## 2. What a good sentence looks like

The reference sites are `gh-dash` and `molt`. Both read the same way:

> Specify the path to a configuration file to use for the dashboard. If the
> configuration file doesn't exist or is invalid, `dash` returns an error.

> A fixpoint release-plan engine bumps dependents across your workspace and rewrites
> their constraints.

Notice: subject, verb, object. A concrete noun in every clause. No adjective doing work
a noun should do. No stance.

### Before and after

> ❌ labdash is a terminal dashboard for GitLab. Merge requests, pipelines, jobs, and
> to-dos from every project and group you touch, in one window that is already full when
> you open it — and there is nothing to configure.

> ✅ labdash is a terminal dashboard for GitLab. It shows the merge requests, pipelines,
> jobs, and to-dos from every project and group you have access to, in one window.

---

> ❌ **Why it cannot merge, in words.** GitLab always knows exactly why a merge request
> is blocked. labdash prints that reason in the row — `conflicts`, `ci failed`,
> `needs 1`, `3 threads open` — so you never open one just to learn why it is waiting.

> ✅ **Merge requests.** Every open merge request across a group and its subgroups, with
> approvals, pipeline status, diff size, and the reason it cannot merge, in one table.

---

> ❌ **Filtering that cannot be mistyped.**

> ✅ **Filtering.**

---

> ❌ The problem is not that GitLab is missing a screen. It is that the screen you need
> is shaped like a query, and every screen GitLab offers is shaped like a project.

> ✅ GitLab organises its pages by project. labdash organises them by what needs you: a
> table of merge requests, pipelines, or to-dos drawn from every project at once.

---

> ❌ Three keys do what a YAML schema used to.

> ✅ Three keys build any view: browse to a scope, filter it, pin the result.

---

## 3. Page shapes

### 3.1 Feature page

```
---
title: <the product noun. "Merge requests". "Job logs". "Pipelines".>
description: <one sentence, what it does, no adjectives>
---

# <the product noun>

<one-paragraph lede: what this view shows and where the data comes from>

<Shot … />

## What you see
<the columns table, the panes, the states. Facts.>

## What you can do
<the key table>

## <one or two specific mechanics worth their own heading>

:::tip Related
<links>
:::
```

Headings are **noun phrases naming the thing**: `Columns`, `The preview pane`,
`Merge trains`, `Paging`. Not `Why this matters`, not `The problem`, not
`How it feels`.

### 3.2 Recipe page

A recipe is a task. Open with the task, in the user's words, as a plain sentence, not a
pull quote. Then the media. Then the steps.

```
---
title: <the task, as a noun phrase. "Release day". "Chasing a red pipeline.">
description: <one sentence>
---

# <title>

<One or two sentences: what you want on screen, and when you would want it.>

<Shot … />          ← the media slot, with a caption describing what happens in it

## Steps
<the numbered key table>

## Why it is built this way
<optional, short, factual — what each tab answers>

## Variations
<optional>
```

Recipes carry media. Every `<Shot>` gets a `caption` that describes what the recording
shows in prose, so the page is complete for a reader who cannot watch it.

### 3.3 Reference page (keys, settings, help)

Tables and short declarative sentences. No lede longer than two lines. gh-dash's
`usage.mdx` is the model.

---

## 4. Naming

| Write | Not |
| --- | --- |
| labdash (always lowercase, even at the start of a sentence) | Labdash, LabDash |
| merge request | MR, in prose. `!2914` is fine in a table or example |
| to-do, to-dos | todo, TODO, Todos |
| settings | config, configuration — except when naming the thing labdash does not have |
| GitLab | Gitlab, gitlab (except in `gitlab.com` and hostnames) |
| self-managed GitLab | self-hosted, on-prem |

Spelling is British-neutral, matching the existing site: `organisation`, `behaviour`,
`colour`. Keep it consistent.

---

## 5. Facts you may state, and where they come from

The source of truth is `research/13-feature-catalog.md`, `research/16-screens-and-flows.md`,
`research/17-design-system.md`, and `research/19-navigation-model.md`. Everything the
docs claim must appear there.

You may state a measured number as a plain fact, with no defence around it:

> A group's merge requests come back in one request. Across 50 projects, 945 pipelines
> take about four seconds.

You may not state it as an argument:

> ❌ measured on the real API, ranked by what you actually work in, in under half a
> second — so labdash never asks.

---

## 6. Configuration: what the docs say

labdash has **no configuration file for the interface**. There are no section
definitions, no column layouts, no template language, and no keybinding file. The
dashboard's contents come from browsing, filtering, and pinning, inside the application.

A small **settings file** holds three things, and only these:

1. **Instances** — which GitLab, and how to reach it: host, API host, subfolder, OAuth
   client id, CA certificate, client certificate, proxy, custom headers.
2. **Your account** — nothing secret. Credentials live in the OS keyring.
3. **Appearance** — theme name, date format, timezone, icon set.

Write about settings the way you would write about a `.ssh/config`: infrastructure the
reader touches once. Do not build a story around its absence. The old copy made "there
is no config file" into the product's headline. It is not the headline. It is a footnote
on the settings page.

Never mention: `mrSections`, YAML schemas for views, JSON Schema, per-section column
width, config layering, shell-command keybindings, or rebinding a built-in key.

---

## 7. Components available in MDX

Registered globally in `rspress.config.ts`. No imports needed.

| Component | Use |
| --- | --- |
| `<Shot id alt caption cols rows />` | A media slot for a screenshot or recording. `alt` required; `caption` strongly encouraged on recipes. |
| `<Terminal title alt caption>` | Hand-drawn ASCII screen, syntax-coloured with the real palette. Use for design-system specimens and small partial screens. |
| `<Keys>b</Keys>` | Key caps. `<Keys>g g</Keys>` renders two caps; `<Keys>Ctrl+K</Keys>` renders one. |
| `<Cards>` / `<Card title href eyebrow>` | Card grid. |
| `<Stats>` / `<Stat value label>` | Measured numbers. |
| `<Palette>` | Design-system colour specimens. |
| `<Wordmark>` | Homepage only. |

Useful classes: `ld-lede` for the opening paragraph, `ld-keytable` for two-column key
tables, `ld-note`, `ld-page` / `ld-hero` / `ld-section` on the custom homepage.

The `<Status>` component and the `status:` / `since:` frontmatter keys are gone. Do not
reintroduce them.

---

## 8. Checklist before saving a page

1. Count the em dashes. Two or fewer.
2. Search the page for `not`, `never`, `no ` at the start of a heading or a card title.
   Rewrite each as a positive statement.
3. Does every `##` heading name a thing? If a heading is a claim, an argument, or a
   question, rename it.
4. Is there any sentence about the state of the project, a version, or the future?
   Delete it.
5. Read the first sentence aloud. If it sounds like a launch tweet, rewrite it as the
   answer to "what is this page about".
6. Every `<Shot>` has an `alt`. Every recipe `<Shot>` has a `caption`.
