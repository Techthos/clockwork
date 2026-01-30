# Clockwork TUI Implementation Plan

## Library Recommendation: tview ✅

After researching Go TUI libraries, **tview** is the best choice for Clockwork:

### Why tview?
- **Perfect widget match**: List, Table, Form, Modal, Pages - all needed components built-in
- **Fastest development**: Pre-built components vs building from scratch
- **Active maintenance**: Used by major projects (K9s, gh CLI, podman-tui)
- **Easy learning curve**: Traditional widget API, chainable methods
- **Form support**: Built-in validation, multiple field types

### Alternative Considered: Bubble Tea + Huh
- More modern Elm architecture (Model-Update-View)
- Steeper learning curve, more code to write
- Better for complex custom components
- **Verdict**: Overkill for CRUD-style TUI

## Architecture Overview

Add TUI as parallel execution mode to existing MCP server:
- `clockwork` → MCP stdio server (default, unchanged)
- `clockwork tui` → New interactive terminal UI

Both modes share the same database and `db.Store` interface.

## Implementation Structure

### New Package: `internal/tui/`

```
internal/tui/
├── app.go           - Main TUI app, page management, global state
├── theme.go         - Color constants and styling
├── projects.go      - Projects list view (table + keyboard shortcuts)
├── project_form.go  - Create/edit project modal forms
├── entries.go       - Entries table view with filtering and summary
├── entry_form.go    - Create/edit entry forms (git + manual modes)
├── stats.go         - Statistics dashboard with breakdowns
├── modals.go        - Reusable confirmation/error dialogs
└── helpers.go       - Shared utilities (formatting, validation)
```

### Modified File

- `cmd/clockwork/main.go` - Add flag parsing for TUI mode

## UI Structure and Navigation

### View Hierarchy

```
Projects View (default)
  ├─ [n] New Project → Project Form
  ├─ [e] Edit Project → Project Form
  ├─ [d] Delete → Confirmation Modal
  ├─ [Enter] View Entries → Entries View (filtered)
  └─ [q] Quit

Entries View
  ├─ [n] New Entry → Mode Selection → Entry Form
  ├─ [e] Edit Entry → Entry Form
  ├─ [d] Delete → Confirmation Modal
  ├─ [i] Toggle invoiced (inline)
  ├─ [f] Configure Filters
  ├─ [s] Statistics → Stats View
  └─ [q] Back to Projects

Statistics View
  ├─ [f] Configure Filters
  ├─ [r] Refresh
  └─ [q] Back to previous view
```

### Key Features Per View

**Projects View**:
- Table: Name, Git Repo Path, Created Date
- Actions: Create, Edit, Delete (with confirmation), View Entries

**Entries View**:
- Table: Date, Duration (formatted), Message, Invoiced (✓/✗)
- Filters: Project dropdown, Date range, Invoiced status
- Summary footer: Total time, Invoiced/Uninvoiced breakdown
- Actions: Create (git/manual), Edit, Delete, Toggle invoiced

**Entry Form - Git Mode**:
- Project dropdown
- Fetches commits since last entry
- Auto-calculates duration from commit timestamps
- Optional override: custom duration, custom message
- Invoiced checkbox, created_at override

**Entry Form - Manual Mode**:
- Project dropdown
- Required duration input (formats: "1h 30m", "90m", "1.5h")
- Required message textarea
- Invoiced checkbox, created_at override

**Statistics View**:
- Total time, entry count
- Invoiced vs uninvoiced breakdown with percentage
- Per-project breakdown with percentage
- Date range display
- Same filters as entries view

## Implementation Phases

### Phase 1: Foundation
1. Add tview dependency (`go get github.com/rivo/tview`)
2. Create `internal/tui/app.go` - App struct with pages, navigation methods
3. Create `internal/tui/theme.go` - Color constants
4. Create `internal/tui/helpers.go` - Duration/date formatting
5. Modify `cmd/clockwork/main.go` - Add flag parsing for `tui` subcommand

### Phase 2: Projects Management
6. Create `internal/tui/modals.go` - Error, confirmation dialogs
7. Create `internal/tui/projects.go` - Projects table view with shortcuts
8. Create `internal/tui/project_form.go` - Create/edit forms with git repo validation
9. Test: Create, edit, delete projects

### Phase 3: Entries Management
10. Create `internal/tui/entries.go` - Entries table with filters and summary
11. Create `internal/tui/entry_form.go` - Git/manual mode forms
12. Implement filter modal (project, date range, invoiced)
13. Test: Create git-based entries, create manual entries, filtering, toggle invoiced

### Phase 4: Statistics and Polish
14. Create `internal/tui/stats.go` - Statistics dashboard
15. Add date picker modal for filters
16. Polish: Loading indicators, error messages, inline validation
17. Refine colors and layout

### Phase 5: Testing and Documentation
18. Integration testing: Navigation flow, edge cases, error handling
19. Update CLAUDE.md and README with TUI usage
20. Performance testing with large datasets

## Critical Files

**To Modify**:
- `/home/alex/Dev/techthos/clockwork/cmd/clockwork/main.go` - Entry point for mode selection

**To Read (Dependencies)**:
- `/home/alex/Dev/techthos/clockwork/internal/db/store.go` - Database interface (all CRUD methods)
- `/home/alex/Dev/techthos/clockwork/internal/models/models.go` - Project, Entry, Statistics structs
- `/home/alex/Dev/techthos/clockwork/internal/git/git.go` - Git functions for commit aggregation
- `/home/alex/Dev/techthos/clockwork/internal/utils/duration.go` - Duration parsing utility

**To Create** (9 new files in `internal/tui/`):
- app.go, theme.go, helpers.go, modals.go
- projects.go, project_form.go
- entries.go, entry_form.go
- stats.go

## Data Flow

```
User Input → tview Event Handler → App Navigation → db.Store → bbolt → Models → View Refresh
```

All database operations use the existing `db.Store` interface - no modifications needed to database layer.

## Global Keyboard Shortcuts

- `Ctrl+C` / `Ctrl+Q` - Quit application
- `Tab` - Navigate within view
- `Esc` - Close modal/cancel
- `↑/↓` - Navigate lists/tables

View-specific shortcuts defined in each view (n, e, d, i, f, s, q).

## Verification Plan

After implementation:

1. **Build and Run**:
   ```bash
   go mod tidy
   go build -o clockwork ./cmd/clockwork
   ./clockwork tui
   ```

2. **Test Workflow**:
   - Create first project with git repo path
   - Create git-based entry (verify commits aggregated)
   - Create manual entry with duration "1h 30m"
   - Edit project and entry
   - Toggle entry invoiced status
   - Filter entries by date range
   - View statistics with project breakdown
   - Delete entry and project (with confirmation)

3. **Edge Cases**:
   - Empty database (first run)
   - Invalid git repo path
   - Invalid duration format
   - No commits since last entry
   - Very long project names/messages

4. **Verify MCP mode still works**:
   ```bash
   ./clockwork  # Should start MCP server as before
   ```

## Dependencies

New dependencies to add:
```go
require (
    github.com/rivo/tview v0.0.0-20240101...
    github.com/gdamore/tcell/v2 v2.7.0  // tview dependency
)
```

Existing dependencies unchanged.

## Estimated Scope

- **New code**: ~2000-2500 lines across 9 files
- **Modified code**: ~20 lines in main.go
- **Development time**: Modular phases allow incremental testing
- **Risk**: Low - reuses existing database layer without modifications

## Notes

- Database locking: bbolt allows one writer at a time - TUI and MCP can't run simultaneously (document this limitation)
- Terminal compatibility: tview works across Linux, macOS, Windows terminals
- State not persisted: Filters reset on TUI restart (acceptable for v1)
- Mouse support: tview supports it by default (bonus feature)
