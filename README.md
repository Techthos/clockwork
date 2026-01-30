<div align="center">

# ⚙️ Clockwork

### *Automated Git-Based Time Tracking*

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![MCP Protocol](https://img.shields.io/badge/MCP-Compatible-green.svg)](https://modelcontextprotocol.io/)
[![Built with tview](https://img.shields.io/badge/TUI-tview-orange.svg)](https://github.com/rivo/tview)

*Track your work time automatically through git commits with both a powerful MCP server and an interactive terminal UI*

[Features](#-features) • [Installation](#-installation) • [Quick Start](#-quick-start) • [Usage](#-usage) • [Documentation](#-documentation)

---

</div>

## 🎯 Overview

**Clockwork** is a dual-mode time tracking system that transforms your git commits into actionable worklog entries. Whether you prefer working through AI assistants (MCP mode) or an interactive terminal interface (TUI mode), Clockwork has you covered.

### 🎭 Two Modes, One Database

**🤖 MCP Server Mode** - Integrate with Claude and other LLM applications
- Expose time tracking as MCP tools
- Natural language interactions: "track the last 2 hours of work"
- Seamless integration with your AI workflow

**💻 TUI Mode** - Interactive terminal interface
- Full-featured terminal UI for direct interaction
- Browse projects and entries with keyboard shortcuts
- Real-time statistics and filtering
- Perfect for quick reviews and manual entries

## ✨ Features

### 📊 Smart Time Tracking
- **Automatic Commit Aggregation** - Collects commits since your last entry
- **Intelligent Duration Calculation** - Estimates work time from commit timestamps
- **Custom Overrides** - Manually adjust duration and messages when needed
- **Flexible Entry Creation** - Git-based or manual entry modes

### 🎨 Rich Terminal UI
- **Projects Dashboard** - Visual overview of all your projects
- **Entries Browser** - Sortable, filterable entry list with summaries
- **Statistics View** - Time breakdowns by project and invoice status
- **Keyboard-Driven** - Efficient navigation without touching the mouse
- **Color-Coded** - Invoiced/uninvoiced status at a glance

### 🔧 Project Management
- **Multi-Project Support** - Track time across unlimited projects
- **Git Repository Integration** - Each project linked to a git repo
- **Invoice Tracking** - Mark entries as invoiced for billing
- **Advanced Filtering** - By project, date range, or invoice status

### 💾 Database & Integration
- **Embedded Database** - bbolt key-value store, no external dependencies
- **Single File Storage** - `~/.local/clockwork/default.db`
- **MCP Protocol** - Works with Claude Desktop, Claude Code, and other MCP clients
- **Data Persistence** - All data safely stored between sessions

## 🚀 Installation

### Prerequisites

- **Go 1.21+** - [Download here](https://go.dev/dl/)
- **Git** - Installed and configured

### Build from Source

```bash
# Clone the repository
git clone https://github.com/techthos/clockwork.git
cd clockwork

# Build the binary
go build -o clockwork ./cmd/clockwork

# Or install to $GOPATH/bin
go install ./cmd/clockwork
```

### Verify Installation

```bash
# The binary supports two modes
./clockwork          # Starts MCP server (default)
./clockwork tui      # Starts terminal UI
```

## ⚡ Quick Start

### 🤖 MCP Server Mode

#### 1. Configure Your MCP Client

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "clockwork": {
      "command": "/absolute/path/to/clockwork"
    }
  }
}
```

**Claude Code CLI** (`~/.claude/config.json`):
```json
{
  "mcpServers": {
    "clockwork": {
      "command": "/absolute/path/to/clockwork"
    }
  }
}
```

💡 **Tip**: Use `$(go env GOPATH)/bin/clockwork` if you ran `go install`

#### 2. Restart Your MCP Client

The Clockwork tools will now be available in your Claude sessions.

#### 3. Start Tracking

```
You: "Create a new project called 'Website Redesign' in /home/user/projects/website"

You: "Track the work I did today on the website project"
```

### 💻 TUI Mode

#### 1. Launch the TUI

```bash
./clockwork tui
```

#### 2. Create Your First Project

- Press `n` to create a new project
- Enter project name and git repository path
- Press Tab to navigate, Enter to save

#### 3. Create Your First Entry

- Press Enter on a project to view its entries
- Press `n` to create a new entry
- Choose Git mode (auto-aggregates commits) or Manual mode
- Fill in the details and save

#### 4. Explore Statistics

- From the entries view, press `s` to see statistics
- View time breakdowns by project and invoice status
- Use `f` to apply filters

## 📖 Usage

### 💻 TUI Mode Reference

#### Global Shortcuts
- `Ctrl+C` or `Ctrl+Q` - Quit application
- `Tab` - Navigate form fields
- `Esc` - Close modal/cancel

#### Projects View
- `n` - New project
- `e` - Edit selected project
- `d` - Delete selected project (with confirmation)
- `Enter` - View project entries
- `q` - Quit application
- `↑/↓` - Navigate list

#### Entries View
- `n` - New entry (choose git or manual mode)
- `e` - Edit selected entry
- `d` - Delete selected entry
- `i` - Toggle invoiced status
- `f` - Configure filters
- `s` - View statistics
- `q` - Back to projects
- `↑/↓` - Navigate list

#### Statistics View
- `f` - Configure filters
- `r` - Refresh statistics
- `q` - Back to entries

#### Entry Creation Modes

**Git Mode** (Automatic):
1. Select project
2. Clockwork fetches commits since last entry
3. Auto-calculates duration from timestamps
4. Auto-generates message from commit summaries
5. Optional: Override duration or message

**Manual Mode**:
1. Select project
2. Enter duration (formats: `1h 30m`, `90m`, `1.5h`)
3. Enter message/description
4. Mark as invoiced (optional)

#### Filtering

Apply filters in entries or statistics views:
- **Project** - Select specific project or "All Projects"
- **Date Range** - Start/end dates (format: YYYY-MM-DD)
- **Invoice Status** - All, Invoiced Only, or Uninvoiced Only

### 🤖 MCP Mode Reference

#### Available Tools

| Tool | Description | Example |
|------|-------------|---------|
| `create_project` | Create a new project | Create project "API Server" at `/code/api` |
| `update_project` | Update project details | Rename project to "API v2" |
| `delete_project` | Delete project and all entries | Delete the API project |
| `list_projects` | List all projects | Show all my projects |
| `create_entry` | Create worklog from git commits | Track 2 hours on the API project |
| `update_entry` | Update entry details | Mark last entry as invoiced |
| `delete_entry` | Delete an entry | Delete yesterday's entry |
| `list_entries` | List project entries with filters | Show uninvoiced entries from last month |

#### Natural Language Examples

```
"Create a new project called 'Mobile App' at /Users/me/code/mobile"

"Track my work today on the mobile app project"

"Show me all uninvoiced time entries"

"Mark the last 3 entries as invoiced"

"How much time did I spend on the API project this week?"

"Create a manual entry for 2 hours of meeting time on the mobile project"
```

#### Programmatic API

```javascript
// Create a project
create_project({
  name: "My Project",
  git_repo_path: "/absolute/path/to/repo"
})

// Create entry from git commits (automatic)
create_entry({
  project_id: "project-uuid",
  invoiced: false
})

// Create manual entry (no git aggregation)
create_entry({
  project_id: "project-uuid",
  duration: 120,  // minutes
  message: "Client meeting and planning",
  invoiced: false
})

// List entries with filters
list_entries({
  project_id: "project-uuid",  // optional, empty = all projects
  start_date: "2025-01-01",    // optional
  end_date: "2025-01-31",      // optional
  invoiced: false              // optional, null = all entries
})

// Update entry
update_entry({
  id: "entry-uuid",
  duration: 180,      // optional
  message: "Updated", // optional
  invoiced: true      // optional
})
```

## 🔧 How It Works

### Commit Aggregation Flow

```
┌─────────────────┐
│  Last Entry     │
│  commit_hash    │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────┐
│  git log <hash>..HEAD       │
│  Fetch new commits          │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│  Aggregate commit messages  │
│  Calculate duration         │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│  Create worklog entry       │
│  Store latest commit hash   │
└─────────────────────────────┘
```

### Duration Calculation Logic

- **Single commit** → Default 30 minutes
- **Multiple commits** → `(last_commit_time - first_commit_time) + 30 minutes`

**Example**: Commits at 9:00 AM and 11:30 AM
- Time span: 2.5 hours
- Buffer: 0.5 hours
- **Total duration: 3 hours (180 minutes)**

### Database Schema

```
~/.local/clockwork/default.db (bbolt)
├── projects/
│   └── <uuid> → {id, name, git_repo_path, created_at, updated_at}
└── entries/
    └── <uuid> → {id, project_id, duration, message, commit_hash, invoiced, created_at, updated_at}
```

## 📊 Data Models

### Project

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Website Redesign",
  "git_repo_path": "/home/user/projects/website",
  "created_at": "2025-01-27T10:00:00Z",
  "updated_at": "2025-01-27T10:00:00Z"
}
```

### Entry

```json
{
  "id": "660e8400-e29b-41d4-a716-446655440001",
  "project_id": "550e8400-e29b-41d4-a716-446655440000",
  "duration": 180,
  "message": "Aggregated 3 commits:\n1. [abc123d] Implement login form\n2. [def456e] Add form validation\n3. [ghi789f] Update styles",
  "commit_hash": "ghi789f...",
  "invoiced": false,
  "created_at": "2025-01-27T14:30:00Z",
  "updated_at": "2025-01-27T14:30:00Z"
}
```

### Statistics

```json
{
  "total_minutes": 540,
  "total_hours": 9.0,
  "entry_count": 3,
  "invoiced_minutes": 180,
  "uninvoiced_minutes": 360,
  "project_breakdown": {
    "project-uuid-1": 300,
    "project-uuid-2": 240
  },
  "earliest_entry": "2025-01-20T09:00:00Z",
  "latest_entry": "2025-01-27T14:30:00Z"
}
```

## 🛠️ Development

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Verbose output
go test ./... -v

# Specific package
go test ./internal/db -v
go test ./internal/git -v
```

### Project Structure

```
clockwork/
├── cmd/
│   └── clockwork/          # Main entry point
├── internal/
│   ├── db/                 # Database operations (bbolt)
│   ├── models/             # Data structures
│   ├── git/                # Git integration
│   ├── server/             # MCP server implementation
│   ├── tui/                # Terminal UI components
│   │   ├── app.go          # Main TUI app structure
│   │   ├── projects.go     # Projects view
│   │   ├── entries.go      # Entries view
│   │   ├── stats.go        # Statistics view
│   │   ├── modals.go       # Dialog boxes
│   │   ├── theme.go        # Color scheme
│   │   └── helpers.go      # Utilities
│   └── utils/              # Shared utilities
├── go.mod
├── go.sum
├── CLAUDE.md               # AI assistant context
└── README.md
```

### Building for Production

```bash
# Build with optimizations
go build -ldflags="-s -w" -o clockwork ./cmd/clockwork

# Build for specific platform
GOOS=linux GOARCH=amd64 go build -o clockwork-linux ./cmd/clockwork
GOOS=darwin GOARCH=arm64 go build -o clockwork-macos ./cmd/clockwork
GOOS=windows GOARCH=amd64 go build -o clockwork.exe ./cmd/clockwork
```

## ⚠️ Important Notes

### Database Locking

**Only one mode can run at a time.** The bbolt database uses a file lock to ensure data integrity.

- ✅ Running TUI mode → MCP mode blocked
- ✅ Running MCP mode → TUI mode blocked
- ❌ Cannot run both simultaneously

**Error**: `failed to open database: timeout`
**Solution**: Stop the other mode before starting a new one

### Git Repository Requirements

- Each project must point to a valid git repository
- Repository must have at least one commit
- Git must be accessible in the system PATH

## 🐛 Troubleshooting

### "No new commits found"

**Cause**: No commits exist between the last entry and HEAD

**Solution**: Make new commits, then create an entry

### "Failed to get git author"

**Cause**: Git not configured properly

**Solution**:
```bash
git config user.name "Your Name"
git config user.email "your@email.com"
```

### "Project not found"

**Cause**: Invalid project ID or project was deleted

**Solution**: List projects to verify:
```bash
# In TUI: View projects screen
# In MCP: Use list_projects tool
```

### "Invalid duration format"

**Cause**: Duration not in recognized format

**Solution**: Use one of these formats:
- `1h 30m` - Hours and minutes
- `90m` - Minutes only
- `1.5h` - Decimal hours
- `90` - Plain number (treated as minutes)

### Database Corruption

**Rare but possible**. If the database becomes corrupted:

```bash
# Backup current database
cp ~/.local/clockwork/default.db ~/.local/clockwork/backup.db

# Remove corrupted database
rm ~/.local/clockwork/default.db

# Clockwork will create a new database on next run
```

## 🤝 Contributing

Contributions are welcome! Here's how:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass (`go test ./...`)
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

### Code Style

- Follow standard Go formatting (`gofmt`)
- Add comments for exported functions
- Keep functions focused and testable
- Update tests for any changes

## 📝 License

MIT License - see [LICENSE](LICENSE) for details

## 🙏 Acknowledgments

Built with these excellent open-source projects:

- **[mcp-go](https://github.com/mark3labs/mcp-go)** - Model Context Protocol implementation by Mark3 Labs
- **[bbolt](https://github.com/etcd-io/bbolt)** - Pure Go embedded key-value database
- **[tview](https://github.com/rivo/tview)** - Rich terminal UI library
- **[tcell](https://github.com/gdamore/tcell)** - Terminal cell-based view library

## 🔗 Links

- **GitHub**: [github.com/techthos/clockwork](https://github.com/techthos/clockwork)
- **MCP Protocol**: [modelcontextprotocol.io](https://modelcontextprotocol.io/)
- **Issues**: [Report bugs or request features](https://github.com/techthos/clockwork/issues)

---

<div align="center">

**Made with ⚙️ and ❤️ by the Techthos team**

*Automate your time tracking. Focus on building great things.*

</div>
