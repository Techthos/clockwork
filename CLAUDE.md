# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Clockwork is a dual-mode time tracking system that automatically tracks work time based on git commits:

1. **MCP Server Mode** - Exposes time tracking as MCP tools for integration with Claude and other LLM applications
2. **TUI Mode** - Interactive terminal user interface for direct interaction with keyboard navigation

Both modes share the same embedded bbolt database and business logic.

**Module Path:** `github.com/techthos/clockwork`

## Build and Run Commands

```bash
# Build binary
go build -o clockwork ./cmd/clockwork

# Install to GOPATH/bin
go install ./cmd/clockwork

# Run TUI mode (default)
./clockwork

# Run MCP server mode
./clockwork mcp

# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/db -v
go test ./internal/git -v
go test ./internal/tui -v

# Tidy dependencies
go mod tidy
```

## MCP Client Setup

After building, configure the clockwork server in your MCP client:

### Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "clockwork": {
      "command": "/absolute/path/to/clockwork",
      "args": ["mcp"]
    }
  }
}
```

### Claude Code CLI

Edit `~/.claude/config.json`:

```json
{
  "mcpServers": {
    "clockwork": {
      "command": "/absolute/path/to/clockwork",
      "args": ["mcp"]
    }
  }
}
```

**Note:** Use the absolute path to the binary. If you ran `go install`, the binary is at `$(go env GOPATH)/bin/clockwork` (typically `~/go/bin/clockwork`).

The server auto-creates its database at `~/.local/clockwork/default.db` on first run.

## Architecture

### Commit Sources

A project's repository is optional, and when present it is not necessarily a
local one. `internal/source` defines the lookup method via `Project.SourceType`:

| Type | Locator | Behavior |
|------|---------|----------|
| `none` | — | No repository. Commit aggregation is refused; manual entries only. |
| `local` | `GitRepoPath` | Clockwork shells out to git itself (the original behavior). |
| `mcp` | `Repository` | Clockwork makes **no** outbound calls. The caller supplies commits in the `commits` argument of `create_entry`. |

Key invariants:

- `source.Resolve(project)` is the only correct way to read a project's type.
  It infers a type for rows written before `source_type` existed (`local` if a
  path is set, else `none`), so legacy databases need no migration. The db
  layer normalizes on read via `normalizeProject`.
- `source.Validate` enforces that exactly the locator matching the type is set.
  It performs no I/O — filesystem checks belong to the server/TUI layer, which
  call `git.IsRepo`.
- `UpdateProject` clears the locator that no longer applies when the type
  changes, so a project can move between methods cleanly.
- The TUI can only drive `local` projects in git mode; there is no client
  present to supply commits for `mcp` ones. `App.localProjects()` filters
  accordingly and the mode selector falls through to manual when none exist.

### Data Flow for Entry Creation

The core workflow aggregates commits into worklog entries:

1. **Resolve the project's source type** (`source.Resolve`) - decides steps 2-3
2. **Retrieve last entry's commit hash** (`store.GetLastCommitHash`) - establishes baseline
3. **Obtain commits since that hash**:
   - `local`: `git.GetCommitsSince` runs `git log <hash>..HEAD`; a baseline that
     no longer exists (rebase, force push) is discarded rather than failing
   - `mcp`: the caller passes them in; absent that, the tool returns an error
     that tells the caller which repository and baseline hash to fetch from
4. **Aggregate commit messages** (`git.AggregateCommits`) - formats into summary
5. **Calculate duration** (`git.CalculateDuration`) - single commit = 30min, multiple = time span + 30min buffer
6. **Store entry with latest commit hash** (`store.CreateEntry`) - becomes next baseline.
   For `local` this is HEAD; for `mcp` it is `git.NewestCommit` of the supplied
   set, since caller-supplied commits arrive in no guaranteed order.

### MCP Tool Registration

`internal/server/server.go` implements 10 MCP tools via the mcp-go library:

- Tool definitions use `mcp.NewTool()` with schema descriptors
- Handlers access arguments via `request.Params.Arguments` (map[string]interface{})
- Required strings extracted via `getRequiredString()` helper
- Errors returned as `mcp.NewToolResultError(string)`
- Success returns `mcp.NewToolResultText(string)` with JSON-marshaled data

**Project tools:** create_project, update_project, delete_project, list_projects, get_commit_baseline
**Entry tools:** create_entry, update_entry, delete_entry, list_entries, get_statistics

`create_project`/`update_project` take an optional `source_type` plus one of
`git_repo_path` or `repository`. `update_project` uses presence-based partial
updates (`db.ProjectUpdate` with `*string` fields), so passing an empty string
clears a field — this is why it does not use the old "non-empty wins" sentinel.

`create_entry` accepts a `commits` array for `mcp` projects. Timestamps are
parsed leniently (RFC3339 or unix seconds); a missing timestamp is only an
error when duration must be derived from the commit span.

### Database Layer

**bbolt** key-value store at `~/.local/clockwork/default.db`:

- Two buckets: `projects` and `entries`
- `CreateProject` takes `db.ProjectInput`; `UpdateProject` takes `db.ProjectUpdate`
- All operations wrapped in transactions (`db.Update`, `db.View`)
- Data stored as JSON-marshaled bytes with UUID keys
- `GetLastEntry()` iterates entries, filters by project_id, returns most recent by created_at
- `DeleteProject()` cascades to all associated entries

### Git Integration

`internal/git/` uses `exec.Command("git", ...)`:

- `GetCommitsSince(repoPath, sinceHash)` - executes `git log --pretty=format:%H|%an|%s|%at [sinceHash..HEAD]`
- Parses pipe-delimited output into `[]models.CommitInfo`
- Empty `sinceHash` returns all commits
- `GetLatestCommitHash()` runs `git rev-parse HEAD`
- All operations require absolute repo paths (`filepath.Abs()`)

### TUI Architecture

`internal/tui/` implements a terminal user interface using tview:

**Main Components:**
- `app.go` - Application shell with page management and navigation
- `projects.go` - Projects list view (table with CRUD operations)
- `entries.go` - Entries list view with filtering and summary footer
- `stats.go` - Statistics dashboard with breakdowns
- `project_form.go` - Project create/edit modal forms
- `entry_form.go` - Entry create/edit with git/manual modes
- `modals.go` - Reusable error/confirm/info dialogs
- `theme.go` - Color scheme constants
- `helpers.go` - Formatting utilities (duration, dates, percentages)

**Navigation Flow:**
```
Projects View (default)
  → Entries View (filtered by project)
    → Statistics View (with filters)
    → Entry Forms (git/manual modes)
  → Project Forms (create/edit)
```

**Keyboard Shortcuts:**
- Global: `Ctrl+C`/`Ctrl+Q` = quit, `Esc` = close modal
- Projects: `n` = new, `e` = edit, `d` = delete, `Enter` = view entries, `q` = quit
- Entries: `n` = new, `e` = edit, `d` = delete, `i` = toggle invoiced, `f` = filter, `s` = stats, `q` = back
- Stats: `f` = filter, `r` = refresh, `q` = back

**Filtering:**
- `FilterOptions` struct tracks current filters (project, date range, invoiced status)
- Filters persist within a session, reset between entries/stats views
- Uses `store.ListEntriesFiltered()` and `store.GetStatistics()` with filter parameters

**TUI vs MCP Mode:**
- Both use same `db.Store` interface - no database layer changes needed
- Entry point (`main.go`) runs the TUI by default and checks for the `mcp` argument to run the server
- Only one mode can run at a time due to bbolt's single-writer file lock

## Key Implementation Details

### MCP Server Initialization

Entry point (`cmd/clockwork/main.go`) → `server.New()`:
1. Resolves `~/.local/clockwork/default.db` path
2. Calls `db.New()` to initialize bbolt store
3. Creates `server.MCPServer` instance ("clockwork", "1.0.0")
4. Registers all 8 tools via `registerTools()`
5. Serves via stdio transport with `server.ServeStdio()`

### Testing Strategy

- Database tests use `t.TempDir()` for isolation
- Git tests use static mock data (no actual git commands)
- Models tests verify struct creation and field access
- No server integration tests (MCP tools tested via manual client interaction)

### Error Handling

- All database errors wrapped with context (`fmt.Errorf(...%w, err)`)
- MCP tool errors converted to strings via `.Error()` method
- Server initialization errors logged to stderr and exit(1)

## Dependencies

**Core:**
- **github.com/mark3labs/mcp-go v0.9.0** - MCP protocol (stdio transport)
- **go.etcd.io/bbolt v1.3.11** - Embedded key-value database
- **github.com/google/uuid v1.6.0** - UUID generation

**TUI:**
- **github.com/rivo/tview v0.42.0** - Terminal UI framework (high-level widgets)
- **github.com/gdamore/tcell/v2 v2.8.1** - Terminal cell-based view (tview dependency)

**System:**
- System **git** command required (not a Go dependency)

## Database Location

Production: `~/.local/clockwork/default.db`
Tests: `t.TempDir()/<testname>.db`

**Important Limitation:** Only one instance can hold the database lock at a time (bbolt limitation). This means:
- Cannot run MCP server and TUI simultaneously
- Attempting to start TUI while MCP server is running will fail with "timeout" error
- Attempting to start MCP server while TUI is running will fail with "timeout" error
- This is by design for data integrity - bbolt ensures single-writer safety

## TUI Usage

After building, launch the TUI:

```bash
./clockwork
```

**Quick Start:**
1. Press `n` in Projects view to create first project
2. Enter project name, pick a Source (`none`/`local`/`mcp`), and fill the matching locator field
3. Press `Enter` on project to view entries
4. Press `n` to create entry (choose Git or Manual mode)
5. Git mode: automatically aggregates commits since last entry
6. Manual mode: enter duration (e.g., "1h 30m") and message
7. Press `s` from entries view to see statistics
8. Press `f` to apply filters (project, date range, invoiced status)

**Entry Creation Modes:**
- **Git Mode**: Fetches commits since last entry, auto-calculates duration, generates message from commit summaries
- **Manual Mode**: User enters duration and message manually (for non-git work like meetings)
