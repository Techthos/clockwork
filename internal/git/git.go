package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/techthos/clockwork/internal/models"
)

// GetAuthor retrieves the git author name from git config
func GetAuthor(repoPath string) (string, error) {
	cmd := exec.Command("git", "config", "user.name")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git author: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCommitsSince retrieves commits from the repository since a specific commit hash
// If sinceHash is empty, retrieves all commits from HEAD
func GetCommitsSince(repoPath, sinceHash string) ([]models.CommitInfo, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve repo path: %w", err)
	}

	// Build git log command
	args := []string{
		"log",
		"--pretty=format:%H|%an|%s|%at",
	}

	if sinceHash != "" {
		args = append(args, fmt.Sprintf("%s..HEAD", sinceHash))
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = absPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git commits: %w", err)
	}

	if len(output) == 0 {
		return []models.CommitInfo{}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	commits := make([]models.CommitInfo, 0, len(lines))

	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) != 4 {
			continue
		}

		timestamp, err := parseUnixTimestamp(parts[3])
		if err != nil {
			continue
		}

		commits = append(commits, models.CommitInfo{
			Hash:      parts[0],
			Author:    parts[1],
			Message:   parts[2],
			Timestamp: timestamp,
		})
	}

	return commits, nil
}

// GetLatestCommitHash retrieves the latest commit hash from the repository
func GetLatestCommitHash(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get latest commit: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// AggregateCommits aggregates multiple commits into a summary message
func AggregateCommits(commits []models.CommitInfo) string {
	if len(commits) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Aggregated %d commits:\n", len(commits)))

	for i, commit := range commits {
		builder.WriteString(fmt.Sprintf("%d. [%s] %s\n",
			i+1,
			commit.Hash[:7],
			commit.Message))
	}

	return builder.String()
}

// CalculateDuration estimates work duration based on commit timestamps
// Uses a simple heuristic: time between first and last commit + 30 minutes
func CalculateDuration(commits []models.CommitInfo) int64 {
	if len(commits) == 0 {
		return 0
	}

	if len(commits) == 1 {
		return 30 // Default 30 minutes for single commit
	}

	// Find earliest and latest commits
	earliest := commits[0].Timestamp
	latest := commits[0].Timestamp

	for _, commit := range commits[1:] {
		if commit.Timestamp.Before(earliest) {
			earliest = commit.Timestamp
		}
		if commit.Timestamp.After(latest) {
			latest = commit.Timestamp
		}
	}

	duration := latest.Sub(earliest)
	minutes := int64(duration.Minutes()) + 30 // Add buffer time

	return minutes
}

func parseUnixTimestamp(ts string) (time.Time, error) {
	var timestamp int64
	_, err := fmt.Sscanf(ts, "%d", &timestamp)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(timestamp, 0), nil
}
