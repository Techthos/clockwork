package git

import (
	"testing"
	"time"

	"github.com/techthos/clockwork/internal/models"
)

func TestAggregateCommits(t *testing.T) {
	commits := []models.CommitInfo{
		{
			Hash:      "abc123def456",
			Author:    "John Doe",
			Message:   "Fix bug in handler",
			Timestamp: time.Now(),
		},
		{
			Hash:      "def456abc123",
			Author:    "John Doe",
			Message:   "Add new feature",
			Timestamp: time.Now(),
		},
	}

	result := AggregateCommits(commits)

	if result == "" {
		t.Error("Expected non-empty aggregation result")
	}

	// Check if result contains both commit messages
	if !contains(result, "Fix bug in handler") {
		t.Error("Expected aggregation to contain first commit message")
	}

	if !contains(result, "Add new feature") {
		t.Error("Expected aggregation to contain second commit message")
	}
}

func TestAggregateCommitsEmpty(t *testing.T) {
	commits := []models.CommitInfo{}
	result := AggregateCommits(commits)

	if result != "" {
		t.Errorf("Expected empty string for no commits, got '%s'", result)
	}
}

func TestCalculateDuration(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		commits  []models.CommitInfo
		expected int64
	}{
		{
			name:     "No commits",
			commits:  []models.CommitInfo{},
			expected: 0,
		},
		{
			name: "Single commit",
			commits: []models.CommitInfo{
				{Hash: "abc", Timestamp: now},
			},
			expected: 30, // Default 30 minutes
		},
		{
			name: "Two commits 1 hour apart",
			commits: []models.CommitInfo{
				{Hash: "abc", Timestamp: now},
				{Hash: "def", Timestamp: now.Add(-1 * time.Hour)},
			},
			expected: 90, // 60 + 30 buffer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateDuration(tt.commits)
			if result != tt.expected {
				t.Errorf("Expected duration %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestCalculateDurationMultipleCommits(t *testing.T) {
	now := time.Now()
	commits := []models.CommitInfo{
		{Hash: "abc", Timestamp: now},
		{Hash: "def", Timestamp: now.Add(-30 * time.Minute)},
		{Hash: "ghi", Timestamp: now.Add(-90 * time.Minute)},
	}

	duration := CalculateDuration(commits)

	// Should be approximately 90 minutes + 30 buffer = 120 minutes
	if duration != 120 {
		t.Errorf("Expected duration around 120 minutes, got %d", duration)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
