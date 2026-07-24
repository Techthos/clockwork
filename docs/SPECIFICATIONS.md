# Clockwork — Specifications

Single source of truth for what this application is. Code and this document change together
(see `.claude/rules/specification-rules.md`).

## Product overview

Clockwork is a local, single-user time tracking system that derives work time from git
commits. It has two surfaces over one embedded database:

1. **MCP server** (`clockwork mcp`) — exposes time tracking as MCP tools over stdio, with
   interactive MCP Apps widgets registered as `ui://` resources for hosts that render them.
2. **TUI** (`clockwork`, the default command) — an interactive terminal interface.

There is no network access, no accounts, no server component. All data lives in one bbolt
file on the local machine.

## Domain model

### Project

| Field | Type | Notes |
|---|---|---|
| `id` | string (UUID) | assigned on create |
| `name` | string | required |
| `source_type` | string enum | `none` / `local` / `mcp`; see Commit sources |
| `git_repo_path` | string | locator for `local`; empty otherwise |
| `repository` | string | locator for `mcp` (e.g. `owner/name` or clone URL); empty otherwise |
| `created_at`, `updated_at` | RFC3339 timestamps | maintained by the store |

### Entry (worklog)

| Field | Type | Notes |
|---|---|---|
| `id` | string (UUID) | assigned on create |
| `project_id` | string | owning project |
| `duration` | integer | minutes |
| `message` | string | free text or aggregated commit summary |
| `commit_hash` | string | the commit this entry stopped at; next entry's baseline |
| `invoiced` | bool | billing flag |
| `created_at`, `updated_at` | RFC3339 timestamps | `created_at` may be supplied by the caller |

### CommitInfo

`hash`, `author`, `message`, `timestamp` — produced by reading a local repository or
supplied by an MCP client for `mcp`-source projects.

## Commit sources

A project's repository is optional; `source_type` selects the lookup method:

| Type | Locator | Behavior |
|---|---|---|
| `none` | — | No repository. Commit aggregation is refused; manual entries only. |
| `local` | `git_repo_path` | Clockwork shells out to the system `git` CLI. |
| `mcp` | `repository` | Clockwork makes no outbound calls; the calling client supplies commits via `create_entry`'s `commits` array. |

Invariants:

- Exactly the locator matching the type is set; switching type clears the other locator.
- Rows written before `source_type` existed are inferred on read: `local` if a path is
  set, else `none`. Legacy databases need no migration.
- `"git"` is accepted as an input alias for `local`.
- Only `local` locators are validated against the filesystem (`git rev-parse`), at the
  server/TUI layer.

## Entry creation flow

1. Resolve the project's source type.
2. Baseline = commit hash of the project's most recent entry.
3. Obtain commits since the baseline:
   - `local`: `git log <baseline>..HEAD`; a baseline that no longer exists (rebase,
     force push) is discarded rather than failing. With no baseline, HEAD alone is used.
   - `mcp`: the caller passes commits in; absent that, the tool returns an error naming
     the repository and baseline hash to fetch from.
   - `none`: refused; manual entries only.
4. Duration: a single commit counts 30 minutes; multiple commits count their time span
   plus a 30 minute buffer. An explicit `duration` overrides the calculation. Commits
   without timestamps are an error when duration must be derived.
5. Message: caller-supplied, else aggregated from commit subjects.
6. The entry stores the newest commit hash as the next baseline (HEAD for `local`;
   newest of the supplied set for `mcp`).

Manual entries skip aggregation entirely: the caller supplies duration (`"1h 30m"` /
`"90m"`) and message; for `local` projects the current HEAD is recorded as baseline when
available.

## Persistence

- **Engine:** bbolt (single file, single writer at a time). Buckets: `projects`, `entries`.
  Values are JSON-marshaled structs keyed by UUID.
- **Location, highest precedence first:** the `--db <path>` flag (accepted by both modes,
  applied in `cmd/`), then `$CLOCKWORK_DB` (an empty value counts as unset), then
  `~/.local/clockwork/default.db`. The default is resolved in exactly one place
  (`db.DefaultPath()`) that every surface and tool goes through. Resolution is pure —
  creating the directory belongs to whatever opens the database. Tests use `t.TempDir()`.
- **Connections:** the store holds the path, not a handle. Each operation opens bbolt
  (read-only for reads, read-write for writes), runs one short transaction and closes, so
  an idle process holds no lock and **the TUI and the MCP server can run at the same time**.
  A collision on the file lock is retried with backoff (75ms attempts, 3s budget) before it
  fails. One read-write open at startup creates the file and its buckets. Rationale and
  limits: `docs/bbolt-concurrent-access-strategy.md`.
- **Change detection:** `Store.TxID()` reports bbolt's committed transaction id, which
  advances on every write, so a reader can detect another process's changes without
  scanning. The TUI reloads on navigation; it does not poll.
- **Cascade:** deleting a project deletes all its entries.

### Entry queries

`list_entries` is backed by a query object (`db.EntryQuery` → `db.EntryPage`), not an
unbounded dump:

- Criteria compose with **AND**; a zero value means "no filter": project, date range,
  invoiced status, and a case-insensitive `search` substring matched against the message,
  the commit hash and the **owning project's name**.
- Ordering: `sort_by` is one of `created_at` (default), `updated_at`, `duration`, `message`,
  always tie-broken on id; `order` is `desc` (default — newest/longest first) or `asc`.
  An unknown sort is rejected before scanning.
- Pagination: 1-based `page`, `page_size` **clamped to 50** (never rejected). The result
  reports `page`, `page_size`, `total`, `total_pages` and `has_more` for the full filtered
  set.
- The scan is in-memory over the entries bucket; there is no per-field index in v1.
  Internal callers that want the whole set unpaginated keep using `ListEntriesFiltered`.

## MCP surface

Server `clockwork` v1.0.0, stdio transport (selected in `cmd/`), tool and resource
capabilities, logging and panic recovery enabled. All logging goes to stderr.

### Tools

| Tool | Kind | Arguments (required in bold) |
|---|---|---|
| `create_project` | CRUD | **`name`**, `source_type`, `git_repo_path`, `repository` (type defaults from whichever locator is given) |
| `update_project` | CRUD | **`id`**, plus presence-based partial fields `name`, `source_type`, `git_repo_path`, `repository` — an explicit empty string clears a field |
| `delete_project` | CRUD | **`id`** — cascades to entries |
| `list_projects` | CRUD | — |
| `get_commit_baseline` | read | **`project_id`** — returns source type, locator, last commit hash, and whether the caller supplies commits |
| `create_entry` | CRUD | **`project_id`**, `message`, `invoiced`, `manual`, `duration`, `created_at`, `commits` (for `mcp` projects; timestamps as RFC3339 or unix seconds) |
| `update_entry` | CRUD | **`id`**, `duration`, `duration_string`, `message`, `commit_hash`, `invoiced`, `created_at` |
| `delete_entry` | CRUD | **`id`** |
| `list_entries` | CRUD | `project_id`, `start_date`, `end_date`, `invoiced` (`true`/`false`/`all`), `search`, `sort_by`, `order`, `page`, `page_size` — returns the rows plus `page`, `page_size`, `total`, `total_pages`, `has_more` |
| `get_statistics` | read | same filters as `list_entries`; returns aggregated totals |
| `new_project_form` | UI read | — renders the create-project form |
| `edit_project_form` | UI read | **`id`** — renders the edit-project form prefilled from the stored project |
| `new_entry_form` | UI read | **`project_id`** — renders the manual-entry form |
| `edit_entry_form` | UI read | **`id`** — renders the edit-entry form prefilled from the stored entry |

Error semantics: user/business failures return MCP error results; only infrastructure
failures surface as protocol errors.

### Interactive widget UI (MCP Apps)

Every CRUD tool also ships an interactive UI built with `github.com/techthos/gadget`,
following the MCP Apps extension (`io.modelcontextprotocol/ui`, spec `2026-01-26`):

- **Six stable `ui://` template resources** are registered once at startup and served
  verbatim from memory as `text/html;profile=mcp-app`: `ui://clockwork/projects`,
  `ui://clockwork/entries`, `ui://clockwork/project-create-form`,
  `ui://clockwork/project-edit-form`, `ui://clockwork/entry-create-form`,
  `ui://clockwork/entry-edit-form`. The documents carry no data.
- **Discovery:** every tool that renders a widget declares it in its tool definition via
  `_meta.ui.resourceUri`. `get_statistics` and `get_commit_baseline` carry no widget and
  return plain JSON.
- **Data:** results are `structuredContent` plus a one-line status sentence — table rows
  under `rows`, form prefill under `values`, inline field errors under `errors`. Every
  mutating tool returns the refreshed collection, so the visible table never goes stale.
  As a fallback for hosts that render result-embedded widgets, each result also carries
  the registered document (same URI, same bytes) as an embedded resource. The
  status + `structuredContent` result always stands alone; widget failures are logged to
  stderr and never fail the tool.

| Widget | Rendered by | Load tool | Contents |
|---|---|---|---|
| Projects table | `list_projects`, `create_project`, `update_project`, `delete_project` | `list_projects` | name, source badge, locators, created date; filterable, paginated; per-row Edit (`edit_project_form`) and Delete (`delete_project`, inline confirmation) |
| Entries table | `list_entries`, `create_entry`, `update_entry`, `delete_entry` | `list_entries` | message, minutes, invoiced badge, project, created datetime; newest first; per-row Edit (`edit_entry_form`) and Delete (`delete_entry`, inline confirmation) |
| Project create form | `new_project_form` | `new_project_form` | name, source select, both locators; submits `create_project` |
| Project edit form | `edit_project_form` | — (id-scoped; prefill arrives with the result) | hidden id plus the same fields, prefilled from the stored project because updates are presence-based and an empty resubmitted field would clear the stored value; submits `update_project` |
| Entry create form | `new_entry_form` | — (project-scoped) | hidden project id, duration, message, invoiced; submits `create_entry` with `manual=true` |
| Entry edit form | `edit_entry_form` | — (id-scoped) | hidden id, duration, message, invoiced; submits `update_entry` |

Validation failures of `create_project`, `update_project` and `create_entry`/`update_entry`
return an error result whose `structuredContent` carries `errors` (keyed by field) and
`values` (what was submitted, overlaid on stored values for updates), so the submitting
form shows the failure inline.

All widget actions and submits target the normal model-visible tools over the standard
App Bridge (`tools/call`, `ui/open-link`, `ui/notifications/size-changed`); no app-only
tools are registered.

## TUI surface

Launched by running `clockwork` with no arguments.

- **Views:** Projects (default) → Entries (per project, with summary footer) →
  Statistics; modal forms for project and entry create/edit; error/confirm/info dialogs.
- **Keys:** global `Ctrl+C`/`Ctrl+Q` quit, `Esc` closes modals. Projects: `n` new,
  `e` edit, `d` delete, `Enter` entries, `q` quit. Entries: `n`/`e`/`d`, `i` toggle
  invoiced, `f` filter, `s` stats, `q` back. Stats: `f` filter, `r` refresh, `q` back.
- **Entry modes:** Git mode aggregates commits since the last entry (only for `local`
  projects — the mode selector falls back to manual when none exist); Manual mode takes
  duration and message.
- **Filters:** project, date range, invoiced status; session-scoped, never persisted.

## Acceptance criteria

- Creating an entry for a `local` project with new commits produces a duration per the
  rules above and advances the baseline to HEAD.
- Creating an entry for an `mcp` project without a `commits` array fails with guidance
  naming the repository and baseline; with commits supplied, the newest supplied hash
  becomes the baseline.
- Creating an entry for a `none` project fails unless `manual=true`.
- `update_project` with an empty-string locator clears that field; switching
  `source_type` clears the non-matching locator.
- Deleting a project removes all its entries.
- The TUI and the MCP server can run simultaneously against one database file: a write
  committed by one is visible to the next operation of the other, and a lock collision is
  retried rather than reported.
- `list_entries` never returns more than 50 rows; an over-max `page_size` is clamped, and
  the returned `total`/`has_more` describe the full filtered set.
- Every widget-backed tool definition carries `_meta.ui.resourceUri` pointing at a
  registered `ui://clockwork/...` resource, and its result carries that widget's data in
  `structuredContent`; a widget render failure never fails the tool call.
- `go test ./... -race` passes; `make build` produces a statically linked binary at
  `bin/clockwork`.
