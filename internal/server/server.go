package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/techthos/clockwork/internal/db"
	"github.com/techthos/clockwork/internal/git"
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

	dbPath := filepath.Join(homeDir, ".local", "time-track", "db")
	store, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create MCP server
	mcpServer := server.NewMCPServer("clockwork", "1.0.0")

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
	val, ok := request.Params.Arguments[key]
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
		args := request.Params.Arguments

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
		mcp.WithDescription("Create a worklog entry with automatic commit aggregation"),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
		mcp.WithString("message", mcp.Description("Custom message (optional, will auto-generate from commits if not provided)")),
		mcp.WithBoolean("invoiced", mcp.Description("Whether the entry has been invoiced (default: false)")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID, err := getRequiredString(request, "project_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := request.Params.Arguments

		customMessage, _ := args["message"].(string)
		invoiced, _ := args["invoiced"].(bool)

		// Get project
		project, err := s.store.GetProject(projectID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project not found: %v", err)), nil
		}

		// Get last entry to determine since commit
		lastEntry, err := s.store.GetLastEntry(projectID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var sinceHash string
		if lastEntry != nil && lastEntry.CommitHash != "" {
			sinceHash = lastEntry.CommitHash
		}

		// Get commits since last entry
		commits, err := git.GetCommitsSince(project.GitRepoPath, sinceHash)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get commits: %v", err)), nil
		}

		if len(commits) == 0 {
			return mcp.NewToolResultError("no new commits found since last entry"), nil
		}

		// Get latest commit hash
		latestHash, err := git.GetLatestCommitHash(project.GitRepoPath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Calculate duration and generate message
		duration := git.CalculateDuration(commits)
		message := customMessage
		if message == "" {
			message = git.AggregateCommits(commits)
		}

		// Create entry
		entry, err := s.store.CreateEntry(projectID, duration, message, latestHash, invoiced)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, _ := json.MarshalIndent(map[string]interface{}{
			"entry":         entry,
			"commits_found": len(commits),
		}, "", "  ")
		return mcp.NewToolResultText(string(result)), nil
	})
}

func (s *ClockworkServer) registerUpdateEntry() {
	tool := mcp.NewTool("update_entry",
		mcp.WithDescription("Update an existing worklog entry"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Entry ID")),
		mcp.WithNumber("duration", mcp.Description("New duration in minutes (optional)")),
		mcp.WithString("message", mcp.Description("New message (optional)")),
		mcp.WithString("commit_hash", mcp.Description("New commit hash (optional)")),
		mcp.WithBoolean("invoiced", mcp.Description("Update invoiced status (optional)")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := getRequiredString(request, "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := request.Params.Arguments

		var duration *int64
		var message, commitHash *string
		var invoiced *bool

		if d, ok := args["duration"].(float64); ok {
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

		entry, err := s.store.UpdateEntry(id, duration, message, commitHash, invoiced)
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
		mcp.WithDescription("List all entries for a project"),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID, err := getRequiredString(request, "project_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		entries, err := s.store.ListEntries(projectID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, _ := json.MarshalIndent(entries, "", "  ")
		return mcp.NewToolResultText(string(result)), nil
	})
}
