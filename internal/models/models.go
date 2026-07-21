package models

import "time"

// Project represents a project and, optionally, the repository its commits
// come from. SourceType selects the lookup method and determines which of the
// two locator fields applies:
//
//	"none"  - no repository; neither locator is set
//	"local" - GitRepoPath points at a git repository on this filesystem
//	"mcp"   - Repository identifies the project to the calling client, which
//	          supplies the commits itself
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	SourceType  string    `json:"source_type"`
	GitRepoPath string    `json:"git_repo_path,omitempty"`
	Repository  string    `json:"repository,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Entry represents a time tracking worklog entry
type Entry struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Duration   int64     `json:"duration"` // Duration in minutes
	Message    string    `json:"message"`
	CommitHash string    `json:"commit_hash,omitempty"` // Optional
	Invoiced   bool      `json:"invoiced"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CommitInfo holds information about a git commit. It is produced either by
// reading a local repository or by a client that looked the commits up itself,
// so it carries JSON tags for transport.
type CommitInfo struct {
	Hash      string    `json:"hash"`
	Author    string    `json:"author"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
