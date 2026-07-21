package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/techthos/clockwork/internal/models"
)

// gitTimeout caps how long any git subprocess may run.
const gitTimeout = 10 * time.Second

// runGit executes git with the given args inside repoPath under a timeout.
func runGit(repoPath string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("git %s timed out after %s", args[0], gitTimeout)
	}
	return out, err
}

// IsRepo reports whether path is a directory that git recognises as a working
// tree. It is the single check every layer should use before treating a path
// as a local commit source.
func IsRepo(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve repo path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path not accessible: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	if _, err := runGit(absPath, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("not a git repository")
	}
	return nil
}

// NewestCommit returns the most recent commit by timestamp. Commits supplied
// by a client arrive in no guaranteed order, so ordering is not assumed.
func NewestCommit(commits []models.CommitInfo) (models.CommitInfo, bool) {
	if len(commits) == 0 {
		return models.CommitInfo{}, false
	}

	newest := commits[0]
	for _, c := range commits[1:] {
		if c.Timestamp.After(newest.Timestamp) {
			newest = c
		}
	}
	return newest, true
}

// GetCommitsSince retrieves commits from the repository since a specific commit hash
// If sinceHash is empty, retrieves all commits from HEAD
func GetCommitsSince(repoPath, sinceHash string) ([]models.CommitInfo, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve repo path: %w", err)
	}

	args := []string{
		"log",
		"--pretty=format:%H|%an|%s|%at",
	}

	if sinceHash != "" {
		args = append(args, fmt.Sprintf("%s..HEAD", sinceHash))
	}

	output, err := runGit(absPath, args...)
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
	output, err := runGit(repoPath, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get latest commit: %w", err)
	}
	hash := strings.TrimSpace(string(output))

	// Validate hash format (40 hex characters)
	if len(hash) != 40 {
		return "", fmt.Errorf("invalid commit hash length: got %d, expected 40", len(hash))
	}
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return "", fmt.Errorf("invalid commit hash: contains non-hex character '%c'", c)
		}
	}

	return hash, nil
}

// GetLatestCommit retrieves the single most recent commit from the repository
func GetLatestCommit(repoPath string) (*models.CommitInfo, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve repo path: %w", err)
	}

	output, err := runGit(absPath, "log", "-1", "--pretty=format:%H|%an|%s|%at")
	if err != nil {
		return nil, fmt.Errorf("failed to get latest commit: %w", err)
	}

	if len(output) == 0 {
		return nil, fmt.Errorf("no commits found")
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) != 4 {
		return nil, fmt.Errorf("unexpected git log output format")
	}

	timestamp, err := parseUnixTimestamp(parts[3])
	if err != nil {
		return nil, fmt.Errorf("failed to parse commit timestamp: %w", err)
	}

	return &models.CommitInfo{
		Hash:      parts[0],
		Author:    parts[1],
		Message:   parts[2],
		Timestamp: timestamp,
	}, nil
}

// ValidateCommitHash checks if a commit hash exists in the repository
func ValidateCommitHash(repoPath, hash string) bool {
	if hash == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "cat-file", "-e", hash)
	cmd.Dir = repoPath
	return cmd.Run() == nil
}

// Heuristic constants for CalculateDuration.
const (
	singleCommitMinutes = 30 // assumed work for a single isolated commit
	bufferMinutes       = 30 // buffer added on top of commit-span time
)

// AggregateCommits aggregates multiple commits into a summary message
func AggregateCommits(commits []models.CommitInfo) string {
	if len(commits) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Aggregated %d commits:\n", len(commits)))

	for i, commit := range commits {
		hashShort := commit.Hash
		if len(hashShort) > 7 {
			hashShort = hashShort[:7]
		}
		builder.WriteString(fmt.Sprintf("%d. [%s] %s\n",
			i+1,
			hashShort,
			commit.Message))
	}

	return builder.String()
}

// CalculateDuration estimates work duration based on commit timestamps
// Uses a simple heuristic: time between first and last commit + buffer
func CalculateDuration(commits []models.CommitInfo) int64 {
	if len(commits) == 0 {
		return 0
	}

	if len(commits) == 1 {
		return singleCommitMinutes
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
	return int64(duration.Minutes()) + bufferMinutes
}

func parseUnixTimestamp(ts string) (time.Time, error) {
	var timestamp int64
	_, err := fmt.Sscanf(ts, "%d", &timestamp)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(timestamp, 0), nil
}
