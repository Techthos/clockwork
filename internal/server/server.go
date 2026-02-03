package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/techthos/clockwork/internal/db"
	"github.com/techthos/clockwork/internal/git"
	"github.com/techthos/clockwork/internal/models"
	"github.com/techthos/clockwork/internal/utils"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ClockworkServer represents the MCP server for time tracking
type ClockworkServer struct {
	store *db.Store
	mcp   *server.MCPServer
}

// New creates a new Clockwork MCP server
func New() (*ClockworkServer, error) {
	// Initialize database
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dbPath := filepath.Join(homeDir, ".local", "clockwork", "default.db")
	store, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"clockwork",
		"1.0.0",
		server.WithInstructions(`Automatically track work time based on git commits of a project.

Examples:
- "track 2h" - Create entry with 2 hours from recent git commits
- "clockwork 1h" - Create entry with 1 hour from recent commits
- "book 1h meeting with alex" - Manual entry without git commit aggregation`),
	)

	cs := &ClockworkServer{
		store: store,
		mcp:   mcpServer,
	}

	// Register tools
	cs.registerTools()

	return cs, nil
}

// Close closes the server and database connection
func (s *ClockworkServer) Close() error {
	return s.store.Close()
}

// Helper function to get required string argument
func getRequiredString(request mcp.CallToolRequest, key string) (string, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid arguments type")
	}
	val, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required argument: %s", key)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("argument %s must be a string", key)
	}
	return str, nil
}

// Serve starts the MCP server using stdio transport
func (s *ClockworkServer) Serve() error {
	return server.ServeStdio(s.mcp)
}

func (s *ClockworkServer) registerTools() {
	// Project tools
	s.registerCreateProject()
	s.registerUpdateProject()
	s.registerDeleteProject()
	s.registerListProjects()

	// Entry tools
	s.registerCreateEntry()
	s.registerUpdateEntry()
	s.registerDeleteEntry()
	s.registerListEntries()
	s.registerGetStatistics()
}

func (s *ClockworkServer) registerCreateProject() {
	tool := mcp.NewTool("create_project",
		mcp.WithDescription("Create a new project for time tracking"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Project name")),
		mcp.WithString("git_repo_path", mcp.Required(), mcp.Description("Path to git repository")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := getRequiredString(request, "name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		gitRepoPath, err := getRequiredString(request, "git_repo_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		project, err := s.store.CreateProject(name, gitRepoPath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, _ := json.MarshalIndent(project, "", "  ")
		return mcp.NewToolResultText(string(result)), nil
	})
}

func (s *ClockworkServer) registerUpdateProject() {
	tool := mcp.NewTool("update_project",
		mcp.WithDescription("Update an existing project"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Project ID")),
		mcp.WithString("name", mcp.Description("New project name (optional)")),
		mcp.WithString("git_repo_path", mcp.Description("New git repository path (optional)")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := getRequiredString(request, "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args, _ := request.Params.Arguments.(map[string]interface{})

		name, _ := args["name"].(string)
		gitRepoPath, _ := args["git_repo_path"].(string)

		project, err := s.store.UpdateProject(id, name, gitRepoPath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, _ := json.MarshalIndent(project, "", "  ")
		return mcp.NewToolResultText(string(result)), nil
	})
}

func (s *ClockworkServer) registerDeleteProject() {
	tool := mcp.NewTool("delete_project",
		mcp.WithDescription("Delete a project and all its entries"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Project ID")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := getRequiredString(request, "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := s.store.DeleteProject(id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Project %s deleted successfully", id)), nil
	})
}

func (s *ClockworkServer) registerListProjects() {
	tool := mcp.NewTool("list_projects",
		mcp.WithDescription("List all projects"),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projects, err := s.store.ListProjects()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, _ := json.MarshalIndent(projects, "", "  ")
		return mcp.NewToolResultText(string(result)), nil
	})
}

func (s *ClockworkServer) registerCreateEntry() {
	tool := mcp.NewTool("create_entry",
		mcp.WithDescription("Create a worklog entry with automatic commit aggregation or manual entry"),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
		mcp.WithString("message", mcp.Description("Custom message (optional, will auto-generate from commits if not provided)")),
		mcp.WithBoolean("invoiced", mcp.Description("Whether the entry has been invoiced (default: false)")),
		mcp.WithBoolean("manual", mcp.Description("Skip git commit aggregation (default: false)")),
		mcp.WithString("duration", mcp.Description("Duration in format '1h 30m' or '90m' (required when manual=true, optional override otherwise)")),
		mcp.WithString("created_at", mcp.Description("Entry creation datetime in RFC3339 format (optional, e.g., '2026-01-15T14:30:00Z')")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID, err := getRequiredString(request, "project_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args, _ := request.Params.Arguments.(map[string]interface{})

		customMessage, _ := args["message"].(string)
		invoiced, _ := args["invoiced"].(bool)
		manual, _ := args["manual"].(bool)
		durationStr, _ := args["duration"].(string)
		createdAtStr, _ := args["created_at"].(string)

		// Parse created_at if provided, otherwise use current time
		createdAt := time.Now()
		if createdAtStr != "" {
			parsed, err := time.Parse(time.RFC3339, createdAtStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid created_at format (use RFC3339, e.g., '2026-01-15T14:30:00Z'): %v", err)), nil
			}
			createdAt = parsed
		}

		// Validate project exists
		_, err = s.store.GetProject(projectID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project not found: %v", err)), nil
		}

		// Manual entry path
		if manual {
			if durationStr == "" {
				return mcp.NewToolResultError("duration is required when manual=true"), nil
			}

			duration, err := utils.ParseDuration(durationStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid duration: %v", err)), nil
			}

			message := customMessage
			if message == "" {
				message = "Manual entry"
			}

			// For manual entries, always store current HEAD commit hash (even if duplicate)
			project, _ := s.store.GetProject(projectID)
			currentHash, err := git.GetLatestCommitHash(project.GitRepoPath)
			if err != nil {
				// If we can't get HEAD hash, just store empty string
				currentHash = ""
			}

			entry, err := s.store.CreateEntry(projectID, duration, message, currentHash, invoiced, createdAt)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			result, _ := json.MarshalIndent(map[string]interface{}{
				"entry": entry,
				"mode":  "manual",
			}, "", "  ")
			return mcp.NewToolResultText(string(result)), nil
		}

		// Git-based entry path
		project, _ := s.store.GetProject(projectID)

		// Find the most recent commit hash across all entries (skips manual entries without one)
		sinceHash, err := s.store.GetLastCommitHash(projectID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Validate that the commit hash still exists in the repository
		if sinceHash != "" && !git.ValidateCommitHash(project.GitRepoPath, sinceHash) {
			sinceHash = ""
		}

		var commits []models.CommitInfo
		if sinceHash != "" {
			commits, err = git.GetCommitsSince(project.GitRepoPath, sinceHash)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get commits: %v", err)), nil
			}
		} else {
			// No baseline — just grab HEAD as a single commit
			commit, err := git.GetLatestCommit(project.GitRepoPath)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get latest commit: %v", err)), nil
			}
			commits = []models.CommitInfo{*commit}
		}

		if len(commits) == 0 {
			return mcp.NewToolResultError("no new commits found since last entry"), nil
		}

		// Get latest commit hash
		latestHash, err := git.GetLatestCommitHash(project.GitRepoPath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Calculate duration (use override if provided)
		var duration int64
		if durationStr != "" {
			duration, err = utils.ParseDuration(durationStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid duration: %v", err)), nil
			}
		} else {
			duration = git.CalculateDuration(commits)
		}

		// Generate message
		message := customMessage
		if message == "" {
			message = git.AggregateCommits(commits)
		}

		// Create entry
		entry, err := s.store.CreateEntry(projectID, duration, message, latestHash, invoiced, createdAt)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, _ := json.MarshalIndent(map[string]interface{}{
			"entry":         entry,
			"commits_found": len(commits),
			"mode":          "git",
		}, "", "  ")
		return mcp.NewToolResultText(string(result)), nil
	})
}

func (s *ClockworkServer) registerUpdateEntry() {
	tool := mcp.NewTool("update_entry",
		mcp.WithDescription("Update an existing worklog entry"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Entry ID")),
		mcp.WithNumber("duration", mcp.Description("New duration in minutes (optional)")),
		mcp.WithString("duration_string", mcp.Description("Duration in format '1h 30m' or '90m' (overrides numeric duration)")),
		mcp.WithString("message", mcp.Description("New message (optional)")),
		mcp.WithString("commit_hash", mcp.Description("New commit hash (optional)")),
		mcp.WithBoolean("invoiced", mcp.Description("Update invoiced status (optional)")),
		mcp.WithString("created_at", mcp.Description("Update entry creation datetime in RFC3339 format (optional, e.g., '2026-01-15T14:30:00Z')")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := getRequiredString(request, "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args, _ := request.Params.Arguments.(map[string]interface{})

		var duration *int64
		var message, commitHash *string
		var invoiced *bool
		var createdAt *time.Time

		// Parse duration_string first (takes priority over numeric duration)
		if durationStr, ok := args["duration_string"].(string); ok && durationStr != "" {
			parsed, err := utils.ParseDuration(durationStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid duration_string: %v", err)), nil
			}
			duration = &parsed
		} else if d, ok := args["duration"].(float64); ok {
			dInt := int64(d)
			duration = &dInt
		}

		if m, ok := args["message"].(string); ok {
			message = &m
		}
		if c, ok := args["commit_hash"].(string); ok {
			commitHash = &c
		}
		if i, ok := args["invoiced"].(bool); ok {
			invoiced = &i
		}

		// Parse created_at if provided
		if createdAtStr, ok := args["created_at"].(string); ok && createdAtStr != "" {
			parsed, err := time.Parse(time.RFC3339, createdAtStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid created_at format (use RFC3339, e.g., '2026-01-15T14:30:00Z'): %v", err)), nil
			}
			createdAt = &parsed
		}

		entry, err := s.store.UpdateEntry(id, duration, message, commitHash, invoiced, createdAt)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, _ := json.MarshalIndent(entry, "", "  ")
		return mcp.NewToolResultText(string(result)), nil
	})
}

func (s *ClockworkServer) registerDeleteEntry() {
	tool := mcp.NewTool("delete_entry",
		mcp.WithDescription("Delete a worklog entry"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Entry ID")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := getRequiredString(request, "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := s.store.DeleteEntry(id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Entry %s deleted successfully", id)), nil
	})
}

func (s *ClockworkServer) registerListEntries() {
	tool := mcp.NewTool("list_entries",
		mcp.WithDescription("List entries with optional filtering"),
		mcp.WithString("project_id", mcp.Description("Project ID (optional, omit for all projects)")),
		mcp.WithString("start_date", mcp.Description("RFC3339 format (optional, e.g., '2026-01-01T00:00:00Z')")),
		mcp.WithString("end_date", mcp.Description("RFC3339 format (optional)")),
		mcp.WithString("invoiced", mcp.Description("Filter: 'true', 'false', or 'all' (default: 'all')")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := request.Params.Arguments.(map[string]interface{})

		projectID, _ := args["project_id"].(string)
		startDateStr, _ := args["start_date"].(string)
		endDateStr, _ := args["end_date"].(string)
		invoicedStr, _ := args["invoiced"].(string)

		// Parse start date
		var startDate *time.Time
		if startDateStr != "" {
			parsed, err := time.Parse(time.RFC3339, startDateStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid start_date format (use RFC3339): %v", err)), nil
			}
			startDate = &parsed
		}

		// Parse end date
		var endDate *time.Time
		if endDateStr != "" {
			parsed, err := time.Parse(time.RFC3339, endDateStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid end_date format (use RFC3339): %v", err)), nil
			}
			endDate = &parsed
		}

		// Validate date range
		if startDate != nil && endDate != nil && startDate.After(*endDate) {
			return mcp.NewToolResultError("start_date must be before end_date"), nil
		}

		// Parse invoiced filter
		var invoicedFilter *bool
		if invoicedStr == "true" {
			val := true
			invoicedFilter = &val
		} else if invoicedStr == "false" {
			val := false
			invoicedFilter = &val
		}

		entries, err := s.store.ListEntriesFiltered(projectID, startDate, endDate, invoicedFilter)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, _ := json.MarshalIndent(entries, "", "  ")
		return mcp.NewToolResultText(string(result)), nil
	})
}

func (s *ClockworkServer) registerGetStatistics() {
	tool := mcp.NewTool("get_statistics",
		mcp.WithDescription("Get aggregated time tracking statistics"),
		mcp.WithString("project_id", mcp.Description("Filter by project (optional)")),
		mcp.WithString("start_date", mcp.Description("RFC3339 format (optional, e.g., '2026-01-01T00:00:00Z')")),
		mcp.WithString("end_date", mcp.Description("RFC3339 format (optional)")),
		mcp.WithString("invoiced", mcp.Description("Filter: 'true', 'false', or 'all' (default: 'all')")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := request.Params.Arguments.(map[string]interface{})

		projectID, _ := args["project_id"].(string)
		startDateStr, _ := args["start_date"].(string)
		endDateStr, _ := args["end_date"].(string)
		invoicedStr, _ := args["invoiced"].(string)

		// Parse start date
		var startDate *time.Time
		if startDateStr != "" {
			parsed, err := time.Parse(time.RFC3339, startDateStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid start_date format (use RFC3339): %v", err)), nil
			}
			startDate = &parsed
		}

		// Parse end date
		var endDate *time.Time
		if endDateStr != "" {
			parsed, err := time.Parse(time.RFC3339, endDateStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid end_date format (use RFC3339): %v", err)), nil
			}
			endDate = &parsed
		}

		// Validate date range
		if startDate != nil && endDate != nil && startDate.After(*endDate) {
			return mcp.NewToolResultError("start_date must be before end_date"), nil
		}

		// Parse invoiced filter
		var invoicedFilter *bool
		if invoicedStr == "true" {
			val := true
			invoicedFilter = &val
		} else if invoicedStr == "false" {
			val := false
			invoicedFilter = &val
		}

		// Get statistics
		stats, err := s.store.GetStatistics(projectID, startDate, endDate, invoicedFilter)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, _ := json.MarshalIndent(stats, "", "  ")
		return mcp.NewToolResultText(string(result)), nil
	})
}
