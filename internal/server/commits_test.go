package server

import (
	"testing"
	"time"

	"github.com/techthos/clockwork/internal/models"
)

func TestParseCommitTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		raw     interface{}
		want    time.Time
		wantErr bool
	}{
		{"rfc3339", "2026-01-15T14:30:00Z", time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC), false},
		{"unix number", float64(1768487400), time.Unix(1768487400, 0), false},
		{"unix string", "1768487400", time.Unix(1768487400, 0), false},
		{"absent", nil, time.Time{}, false},
		{"empty string", "", time.Time{}, false},
		{"garbage", "not a time", time.Time{}, true},
		{"wrong type", true, time.Time{}, true},
	}

	for _, tt := range tests {
		got, err := parseCommitTimestamp(tt.raw)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s: expected error, got %v", tt.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.name, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestParseSuppliedCommits(t *testing.T) {
	raw := []interface{}{
		map[string]interface{}{
			"hash":      "abc123",
			"author":    "Alex",
			"message":   "feat: add source types",
			"timestamp": "2026-01-15T14:30:00Z",
		},
		map[string]interface{}{
			"hash":      "def456",
			"message":   "fix: handle empty commits",
			"timestamp": float64(1768487400),
		},
	}

	commits, err := parseSuppliedCommits(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	if commits[0].Hash != "abc123" || commits[0].Author != "Alex" {
		t.Errorf("first commit not parsed: %+v", commits[0])
	}
	if commits[1].Author != "" {
		t.Errorf("missing author should stay empty, got %q", commits[1].Author)
	}
}

func TestParseSuppliedCommitsRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		raw  interface{}
	}{
		{"not an array", map[string]interface{}{"hash": "abc"}},
		{"element not an object", []interface{}{"abc123"}},
		{"missing message", []interface{}{map[string]interface{}{"hash": "abc123"}}},
		{"blank message", []interface{}{map[string]interface{}{"message": "   "}}},
		{"bad timestamp", []interface{}{map[string]interface{}{"message": "x", "timestamp": "yesterday"}}},
	}

	for _, tt := range tests {
		if _, err := parseSuppliedCommits(tt.raw); err == nil {
			t.Errorf("%s: expected error, got nil", tt.name)
		}
	}
}

func TestMissingTimestamps(t *testing.T) {
	now := time.Now()
	withTime := []models.CommitInfo{{Message: "a", Timestamp: now}, {Message: "b", Timestamp: now}}
	if missingTimestamps(withTime) {
		t.Error("commits with timestamps should not be flagged")
	}

	partial := []models.CommitInfo{{Message: "a", Timestamp: now}, {Message: "b"}}
	if !missingTimestamps(partial) {
		t.Error("a commit without a timestamp should be flagged")
	}
}

func TestOptionalString(t *testing.T) {
	args := map[string]interface{}{
		"present": "value",
		"empty":   "",
		"wrong":   42,
	}

	if got := optionalString(args, "absent"); got != nil {
		t.Errorf("absent key should return nil, got %q", *got)
	}
	if got := optionalString(args, "present"); got == nil || *got != "value" {
		t.Errorf("present key not returned correctly: %v", got)
	}
	// An empty string is a clear request, not an absent one.
	if got := optionalString(args, "empty"); got == nil || *got != "" {
		t.Errorf("empty string should be distinguishable from absent: %v", got)
	}
	if got := optionalString(args, "wrong"); got != nil {
		t.Errorf("non-string should return nil, got %q", *got)
	}
}
