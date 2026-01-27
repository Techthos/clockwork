package models

import "time"

// Project represents a project with associated git repository
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	GitRepoPath string    `json:"git_repo_path"`
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

// CommitInfo holds information about a git commit
type CommitInfo struct {
	Hash      string
	Author    string
	Message   string
	Timestamp time.Time
}
