package models

import (
	"testing"
	"time"
)

func TestProjectCreation(t *testing.T) {
	project := &Project{
		ID:          "test-id",
		Name:        "Test Project",
		GitRepoPath: "/path/to/repo",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if project.Name != "Test Project" {
		t.Errorf("Expected name 'Test Project', got '%s'", project.Name)
	}

	if project.GitRepoPath != "/path/to/repo" {
		t.Errorf("Expected git repo path '/path/to/repo', got '%s'", project.GitRepoPath)
	}
}

func TestEntryCreation(t *testing.T) {
	entry := &Entry{
		ID:         "test-entry-id",
		ProjectID:  "test-project-id",
		Duration:   120,
		Message:    "Test work",
		CommitHash: "abc123",
		Invoiced:   false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if entry.Duration != 120 {
		t.Errorf("Expected duration 120, got %d", entry.Duration)
	}

	if entry.Invoiced != false {
		t.Errorf("Expected invoiced to be false, got %v", entry.Invoiced)
	}
}

func TestCommitInfo(t *testing.T) {
	commit := CommitInfo{
		Hash:      "abc123def456",
		Author:    "John Doe",
		Message:   "Fix bug in handler",
		Timestamp: time.Now(),
	}

	if commit.Hash != "abc123def456" {
		t.Errorf("Expected hash 'abc123def456', got '%s'", commit.Hash)
	}

	if commit.Author != "John Doe" {
		t.Errorf("Expected author 'John Doe', got '%s'", commit.Author)
	}
}
