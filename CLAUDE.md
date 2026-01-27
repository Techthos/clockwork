# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Clockwork is an MCP (Model Context Protocol) server that automatically tracks work time based on git commits. It aggregates commits into worklog entries and calculates durations.

**Module Path:** `github.com/techthos/clockwork`

## Build and Test Commands

```bash
# Build binary
go build -o clockwork ./cmd/clockwork

# Install to GOPATH/bin
go install ./cmd/clockwork

# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/db -v
go test ./internal/git -v

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
      "command": "/absolute/path/to/clockwork"
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
      "command": "/absolute/path/to/clockwork"
    }
  }
}
```

**Note:** Use the absolute path to the binary. If you ran `go install`, the binary is at `$(go env GOPATH)/bin/clockwork` (typically `~/go/bin/clockwork`).

The server auto-creates its database at `~/.local/clockwork/default.db` on first run.

## Architecture

### Data Flow for Entry Creation

The core workflow aggregates git commits into worklog entries:

1. **Retrieve last entry's commit hash** (`store.GetLastEntry`) - establishes baseline
2. **Fetch commits since that hash** (`git.GetCommitsSince`) - uses `git log <hash>..HEAD`
3. **Aggregate commit messages** (`git.AggregateCommits`) - formats into summary
4. **Calculate duration** (`git.CalculateDuration`) - single commit = 30min, multiple = time span + 30min buffer
5. **Store entry with latest commit hash** (`store.CreateEntry`) - becomes next baseline

### MCP Tool Registration

`internal/server/server.go` implements 8 MCP tools via the mcp-go library (v0.9.0):

- Tool definitions use `mcp.NewTool()` with schema descriptors
- Handlers access arguments via `request.Params.Arguments` (map[string]interface{})
- Required strings extracted via `getRequiredString()` helper
- Errors returned as `mcp.NewToolResultError(string)`
- Success returns `mcp.NewToolResultText(string)` with JSON-marshaled data

**Project tools:** create_project, update_project, delete_project, list_projects
**Entry tools:** create_entry, update_entry, delete_entry, list_entries

### Database Layer

**bbolt** key-value store at `~/.local/clockwork/default.db`:

- Two buckets: `projects` and `entries`
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

- **github.com/mark3labs/mcp-go v0.9.0** - MCP protocol (stdio transport)
- **go.etcd.io/bbolt v1.3.11** - Embedded key-value database
- **github.com/google/uuid v1.6.0** - UUID generation
- System **git** command required (not a Go dependency)

## Database Location

Production: `~/.local/clockwork/default.db`
Tests: `t.TempDir()/<testname>.db`

Only one instance can hold the database lock at a time (bbolt limitation).
