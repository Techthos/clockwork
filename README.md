# Clockwork - Automated Git-Based Time Tracking MCP Server

Clockwork is a Model Context Protocol (MCP) server that automatically tracks your work time based on git commits. It aggregates commits into worklog entries, calculates durations, and manages projects - all through a simple CLI interface.

## Features

- **Automatic Commit Aggregation**: Automatically collects commits since your last worklog entry
- **Smart Duration Calculation**: Estimates work time based on commit timestamps
- **Project Management**: Track multiple projects with associated git repositories
- **Invoice Tracking**: Mark entries as invoiced for billing purposes
- **Embedded Database**: Standalone bbolt database requiring no external services
- **MCP Protocol**: Integrates seamlessly with LLM applications like Claude

## Installation

### Prerequisites

- Go 1.21 or higher
- Git installed and configured

### Build from Source

```bash
git clone https://github.com/alex20465/clockwork-mcp.git
cd clockwork-mcp
go build -o clockwork ./cmd/clockwork
```

### Install

```bash
go install ./cmd/clockwork
```

## Configuration

### MCP Client Setup

Add to your MCP client configuration (e.g., Claude Desktop):

```json
{
  "mcpServers": {
    "clockwork": {
      "command": "/path/to/clockwork"
    }
  }
}
```

The server will automatically create its database at `~/.local/clockwork/default.db`.

## Usage

### Creating a Project

```javascript
// Using MCP tool
create_project({
  name: "My Project",
  git_repo_path: "/path/to/my/repo"
})
```

### Logging Work Time

```javascript
// Automatically aggregates commits and calculates duration
create_entry({
  project_id: "project-uuid",
  message: "Optional custom message",  // If omitted, generates from commits
  invoiced: false
})
```

This will:
1. Find all commits since your last worklog entry
2. Aggregate commit messages into a summary
3. Calculate work duration based on commit timestamps
4. Create an entry with the latest commit hash

### Listing Projects

```javascript
list_projects()
```

### Listing Entries

```javascript
list_entries({
  project_id: "project-uuid"
})
```

### Updating an Entry

```javascript
update_entry({
  id: "entry-uuid",
  duration: 180,  // Override calculated duration (minutes)
  message: "Updated work description",
  invoiced: true
})
```

### Deleting Resources

```javascript
delete_project({ id: "project-uuid" })  // Deletes project and all entries
delete_entry({ id: "entry-uuid" })
```

## Data Models

### Project

```json
{
  "id": "uuid",
  "name": "Project Name",
  "git_repo_path": "/absolute/path/to/repo",
  "created_at": "2025-01-27T10:00:00Z",
  "updated_at": "2025-01-27T10:00:00Z"
}
```

### Entry

```json
{
  "id": "uuid",
  "project_id": "project-uuid",
  "duration": 120,
  "message": "Aggregated 3 commits:\n1. [abc123] Fix bug\n2. [def456] Add feature\n3. [ghi789] Update docs",
  "commit_hash": "ghi789...",
  "invoiced": false,
  "created_at": "2025-01-27T10:00:00Z",
  "updated_at": "2025-01-27T10:00:00Z"
}
```

## How It Works

### Commit Aggregation

When you create an entry, Clockwork:

1. **Finds the baseline**: Looks up the last entry's commit hash
2. **Retrieves commits**: Gets all commits from that hash to HEAD
3. **Generates summary**: Creates a formatted list of commit messages
4. **Calculates time**: Estimates duration based on commit timestamps
5. **Stores reference**: Saves the latest commit hash for next time

### Duration Calculation

- **Single commit**: Default 30 minutes
- **Multiple commits**: Time between first and last commit + 30 minute buffer

Example: If you have commits at 9:00 AM and 11:30 AM, the calculated duration is 2.5 hours + 0.5 hours = 3 hours (180 minutes).

## Database

Clockwork uses **bbolt**, a pure Go embedded key-value database:

- **Location**: `~/.local/clockwork/default.db`
- **Format**: Single file, no external dependencies
- **Buckets**: `projects` and `entries`
- **Persistence**: All data persists between runs

## MCP Tools Reference

| Tool | Description | Required Parameters |
|------|-------------|---------------------|
| `create_project` | Create a new project | name, git_repo_path |
| `update_project` | Update project details | id, [name], [git_repo_path] |
| `delete_project` | Delete project and entries | id |
| `list_projects` | List all projects | - |
| `create_entry` | Create worklog with auto-aggregation | project_id, [message], [invoiced] |
| `update_entry` | Update entry fields | id, [duration], [message], [commit_hash], [invoiced] |
| `delete_entry` | Delete an entry | id |
| `list_entries` | List project entries | project_id |

## Development

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/db -v
```

### Project Structure

```
clockwork-mcp/
├── cmd/clockwork/          # Main entry point
├── internal/
│   ├── db/                 # Database operations
│   ├── models/             # Data structures
│   ├── git/                # Git integration
│   └── server/             # MCP server implementation
├── docs/                   # Documentation
├── go.mod
├── CLAUDE.md               # Project context for Claude
└── README.md
```

## Troubleshooting

### "No new commits found"

This means there are no commits since your last logged entry. Make some commits first, then create a new entry.

### "Failed to get git author"

Ensure git is configured:
```bash
git config user.name "Your Name"
git config user.email "your@email.com"
```

### "Project not found"

Verify the project exists:
```javascript
list_projects()
```

### Database Locked

Only one instance of Clockwork can run at a time. Close other instances or check for stale processes.

## License

MIT

## Contributing

Contributions welcome! Please open an issue or pull request.

## Acknowledgments

- Built with [mcp-go](https://github.com/mark3labs/mcp-go) by Mark3 Labs
- Database powered by [bbolt](https://github.com/etcd-io/bbolt)

---

**Made with ⚙️ by the Techthos team**
