package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/techthos/clockwork/internal/db"
	"github.com/techthos/clockwork/internal/git"
	"github.com/techthos/clockwork/internal/models"
	"github.com/techthos/clockwork/internal/source"
	"github.com/techthos/clockwork/internal/utils"
)

// ClockworkServer represents the MCP server for time tracking
type ClockworkServer struct {
	store *db.Store
	mcp   *server.MCPServer
}

// New creates a new Clockwork MCP server
func New() (*ClockworkServer, error) {
	dbPath, err := db.DefaultPath()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve database path: %w", err)
	}

	store, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"clockwork",
		"1.0.0",
		server.WithInstructions(`Automatically track work time based on the commits of a project.

A repository is optional. Each project declares how its commits are found via
source_type:
- "none"  - no repository; only manual entries are possible
- "local" - a git repository on this machine; clockwork reads it itself
- "mcp"   - clockwork cannot see the repository. You look the commits up (via a
            repository MCP server, an API, or any other means) and pass them to
            create_entry in the "commits" array. Call get_commit_baseline first
            to learn which commit to start from.

Examples:
- "track 2h" - Create entry with 2 hours from recent commits
- "clockwork 1h" - Create entry with 1 hour from recent commits
- "book 1h meeting with alex" - Manual entry without commit aggregation`),
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
	if str == "" {
		return "", fmt.Errorf("argument %s must not be empty", key)
	}
	return str, nil
}

// argsMap returns the request arguments as a map (empty map if missing/wrong type).
func argsMap(request mcp.CallToolRequest) map[string]interface{} {
	if m, ok := request.Params.Arguments.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

// optionalString returns a pointer to the value at key when the caller sent
// that key, and nil when it was omitted. The distinction matters for partial
// updates: an empty string is a request to clear a field.
func optionalString(args map[string]interface{}, key string) *string {
	raw, ok := args[key]
	if !ok {
		return nil
	}
	str, ok := raw.(string)
	if !ok {
		return nil
	}
	return &str
}

// parseCommitTimestamp accepts either an RFC3339 string or unix seconds, since
// different repository APIs report commit times differently.
func parseCommitTimestamp(raw interface{}) (time.Time, error) {
	switch v := raw.(type) {
	case nil:
		return time.Time{}, nil
	case float64:
		return time.Unix(int64(v), 0), nil
	case string:
		if v == "" {
			return time.Time{}, nil
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t, nil
		}
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Unix(secs, 0), nil
		}
		return time.Time{}, fmt.Errorf("expected RFC3339 or unix seconds, got %q", v)
	default:
		return time.Time{}, fmt.Errorf("expected RFC3339 string or unix seconds, got %T", raw)
	}
}

// parseSuppliedCommits converts the caller-provided commits array into
// CommitInfo. Used when a project's commits are looked up by the client rather
// than read from disk.
func parseSuppliedCommits(raw interface{}) ([]models.CommitInfo, error) {
	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("commits must be an array")
	}

	commits := make([]models.CommitInfo, 0, len(list))
	for i, item := range list {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("commits[%d] must be an object", i)
		}

		message, _ := obj["message"].(string)
		if strings.TrimSpace(message) == "" {
			return nil, fmt.Errorf("commits[%d].message is required", i)
		}
		hash, _ := obj["hash"].(string)
		author, _ := obj["author"].(string)

		timestamp, err := parseCommitTimestamp(obj["timestamp"])
		if err != nil {
			return nil, fmt.Errorf("commits[%d].timestamp: %w", i, err)
		}

		commits = append(commits, models.CommitInfo{
			Hash:      hash,
			Author:    author,
			Message:   message,
			Timestamp: timestamp,
		})
	}
	return commits, nil
}

// jsonResult marshals v to indented JSON and wraps it in an MCP tool result.
// If marshaling fails, an error result is returned instead of silently
// emitting invalid JSON.
func jsonResult(v interface{}) *mcp.CallToolResult {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %v", err))
	}
	return mcp.NewToolResultText(string(data))
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

	s.registerGetCommitBaseline()

	// Entry tools
	s.registerCreateEntry()
	s.registerUpdateEntry()
	s.registerDeleteEntry()
	s.registerListEntries()
	s.registerGetStatistics()
}

// sourceTypeDescription documents the lookup methods for the tool schemas.
func sourceTypeDescription() string {
	parts := make([]string, 0, len(source.All()))
	for _, t := range source.All() {
		parts = append(parts, fmt.Sprintf("'%s' (%s)", t, source.Describe(t)))
	}
	return "How commits are looked up for this project: " + strings.Join(parts, ", ")
}

func (s *ClockworkServer) registerCreateProject() {
	tool := mcp.NewTool("create_project",
		mcp.WithDescription("Create a new project for time tracking. A repository is optional: projects with source_type 'none' track manual entries only."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Project name")),
		mcp.WithString("source_type",
			mcp.Enum(string(source.None), string(source.Local), string(source.MCP)),
			mcp.Description(sourceTypeDescription()+". Defaults to 'local' if git_repo_path is given, 'mcp' if repository is given, otherwise 'none'."),
		),
		mcp.WithString("git_repo_path", mcp.Description("Path to a git repository on this filesystem. Only for source_type 'local'.")),
		mcp.WithString("repository", mcp.Description("Repository identifier you will use to look up commits yourself, e.g. 'owner/name' or a clone URL. Only for source_type 'mcp'.")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := getRequiredString(request, "name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := argsMap(request)

		sourceType, _ := args["source_type"].(string)
		gitRepoPath, _ := args["git_repo_path"].(string)
		repository, _ := args["repository"].(string)

		resolved := source.Infer(gitRepoPath, repository)
		if sourceType != "" {
			parsed, err := source.Parse(sourceType)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			resolved = parsed
		}

		// A local source is the only one clockwork reads itself, so it is the
		// only one whose locator can be checked here.
		if resolved == source.Local {
			if err := git.IsRepo(gitRepoPath); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("git_repo_path is not usable: %v", err)), nil
			}
		}

		project, err := s.store.CreateProject(db.ProjectInput{
			Name:        name,
			SourceType:  resolved.String(),
			GitRepoPath: gitRepoPath,
			Repository:  repository,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(project), nil
	})
}

func (s *ClockworkServer) registerUpdateProject() {
	tool := mcp.NewTool("update_project",
		mcp.WithDescription("Update an existing project. Switching source_type clears the locator that no longer applies."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Project ID")),
		mcp.WithString("name", mcp.Description("New project name (optional)")),
		mcp.WithString("source_type",
			mcp.Enum(string(source.None), string(source.Local), string(source.MCP)),
			mcp.Description(sourceTypeDescription()+" (optional)"),
		),
		mcp.WithString("git_repo_path", mcp.Description("New git repository path (optional). Pass an empty string to clear it.")),
		mcp.WithString("repository", mcp.Description("New repository identifier (optional). Pass an empty string to clear it.")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := getRequiredString(request, "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := argsMap(request)

		// Presence, not emptiness, decides what gets written — that is what
		// makes clearing a locator possible.
		upd := db.ProjectUpdate{
			Name:        optionalString(args, "name"),
			SourceType:  optionalString(args, "source_type"),
			GitRepoPath: optionalString(args, "git_repo_path"),
			Repository:  optionalString(args, "repository"),
		}
		if upd.Name == nil && upd.SourceType == nil && upd.GitRepoPath == nil && upd.Repository == nil {
			return mcp.NewToolResultError("at least one of 'name', 'source_type', 'git_repo_path' or 'repository' must be provided"), nil
		}

		if upd.GitRepoPath != nil && *upd.GitRepoPath != "" {
			if err := git.IsRepo(*upd.GitRepoPath); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("git_repo_path is not usable: %v", err)), nil
			}
		}

		project, err := s.store.UpdateProject(id, upd)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(project), nil
	})
}

// registerGetCommitBaseline exposes what a client needs in order to look
// commits up on clockwork's behalf: the lookup method, the repository
// identifier, and the commit the last entry stopped at.
func (s *ClockworkServer) registerGetCommitBaseline() {
	tool := mcp.NewTool("get_commit_baseline",
		mcp.WithDescription("Get a project's commit lookup method and the commit hash the last entry stopped at. For source_type 'mcp', call this first, fetch the commits after that hash yourself, then pass them to create_entry."),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID, err := getRequiredString(request, "project_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		project, err := s.store.GetProject(projectID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project not found: %v", err)), nil
		}

		lastHash, err := s.store.GetLastCommitHash(projectID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		srcType := source.Resolve(project)
		return jsonResult(map[string]interface{}{
			"project_id":                 project.ID,
			"project_name":               project.Name,
			"source_type":                srcType.String(),
			"repository":                 source.Locator(project),
			"last_commit_hash":           lastHash,
			"commits_supplied_by_caller": srcType.SuppliesCommits(),
		}), nil
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

		return jsonResult(projects), nil
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
		mcp.WithArray("commits",
			mcp.Description("Commits you looked up yourself, for projects with source_type 'mcp'. Clockwork does not fetch these — call get_commit_baseline, retrieve the commits after that hash with your own repository tools, and pass them here. Ignored for source_type 'local'."),
			mcp.Items(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"hash":      map[string]interface{}{"type": "string", "description": "Commit hash"},
					"author":    map[string]interface{}{"type": "string", "description": "Commit author name"},
					"message":   map[string]interface{}{"type": "string", "description": "Commit subject line"},
					"timestamp": map[string]interface{}{"type": "string", "description": "Commit time as RFC3339 or unix seconds"},
				},
				"required": []string{"message"},
			}),
		),
	)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID, err := getRequiredString(request, "project_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args := argsMap(request)

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

		// Resolve project once (single source of truth for the rest of the handler).
		project, err := s.store.GetProject(projectID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project not found: %v", err)), nil
		}

		srcType := source.Resolve(project)

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

			// Only a local repository can be probed for HEAD, and even then the
			// entry stands on its own if git is unavailable.
			currentHash := ""
			if srcType == source.Local {
				hash, hashErr := git.GetLatestCommitHash(project.GitRepoPath)
				if hashErr != nil {
					fmt.Fprintf(os.Stderr, "warning: could not read git HEAD for manual entry (project %s): %v\n", projectID, hashErr)
				} else {
					currentHash = hash
				}
			}

			entry, err := s.store.CreateEntry(projectID, duration, message, currentHash, invoiced, createdAt)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return jsonResult(map[string]interface{}{
				"entry": entry,
				"mode":  "manual",
			}), nil
		}

		// Commit-based entry path. Where the commits come from depends entirely
		// on the project's lookup method.
		if srcType == source.None {
			return mcp.NewToolResultError(fmt.Sprintf(
				"project %q has no repository (source_type=none), so commits cannot be aggregated. Create the entry with manual=true and an explicit duration, or set a source with update_project.",
				project.Name)), nil
		}

		// Baseline: the commit the previous entry stopped at.
		sinceHash, err := s.store.GetLastCommitHash(projectID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var commits []models.CommitInfo
		var latestHash string

		if srcType.SuppliesCommits() {
			rawCommits, provided := args["commits"]
			if !provided {
				return mcp.NewToolResultError(fmt.Sprintf(
					"project %q uses source_type=mcp: clockwork does not look commits up itself. Fetch the commits for repository %q%s using your own repository tools, then call create_entry again with them in the 'commits' array. Alternatively pass manual=true with an explicit duration.",
					project.Name, project.Repository, sinceDescription(sinceHash))), nil
			}

			commits, err = parseSuppliedCommits(rawCommits)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(commits) == 0 {
				return mcp.NewToolResultError("the 'commits' array is empty: nothing to aggregate"), nil
			}
			if newest, ok := git.NewestCommit(commits); ok {
				latestHash = newest.Hash
			}
		} else {
			// Local repository: a baseline that no longer exists (rebase, force
			// push, fresh clone) is discarded rather than failing the call.
			if sinceHash != "" && !git.ValidateCommitHash(project.GitRepoPath, sinceHash) {
				sinceHash = ""
			}

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

			latestHash, err = git.GetLatestCommitHash(project.GitRepoPath)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		// Calculate duration (use override if provided)
		var duration int64
		if durationStr != "" {
			duration, err = utils.ParseDuration(durationStr)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid duration: %v", err)), nil
			}
		} else {
			if missingTimestamps(commits) {
				return mcp.NewToolResultError("cannot calculate duration: one or more commits have no timestamp. Supply 'timestamp' on each commit (RFC3339 or unix seconds), or pass an explicit 'duration'."), nil
			}
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

		return jsonResult(map[string]interface{}{
			"entry":         entry,
			"commits_found": len(commits),
			"mode":          "git",
			"source_type":   srcType.String(),
		}), nil
	})
}

// sinceDescription renders the baseline commit for the guidance returned to a
// client that has to fetch commits itself.
func sinceDescription(sinceHash string) string {
	if sinceHash == "" {
		return " (no previous entry exists, so use the most recent commits)"
	}
	return fmt.Sprintf(" made after commit %s", sinceHash)
}

// missingTimestamps reports whether any commit lacks the timestamp that
// duration calculation depends on.
func missingTimestamps(commits []models.CommitInfo) bool {
	for _, c := range commits {
		if c.Timestamp.IsZero() {
			return true
		}
	}
	return false
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
		args := argsMap(request)

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

		if duration == nil && message == nil && commitHash == nil && invoiced == nil && createdAt == nil {
			return mcp.NewToolResultError("no fields provided to update"), nil
		}

		entry, err := s.store.UpdateEntry(id, duration, message, commitHash, invoiced, createdAt)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return jsonResult(entry), nil
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
		args := argsMap(request)

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

		return jsonResult(entries), nil
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
		args := argsMap(request)

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

		return jsonResult(stats), nil
	})
}
