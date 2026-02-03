package main

import (
	"fmt"
	"os/exec"

	"github.com/techthos/clockwork/internal/git"
)

func main() {
	repoPath := "/home/alex/Dev/quotelier/qx"

	// Test 1: Get latest commit hash
	fmt.Println("=== Test 1: GetLatestCommitHash ===")
	latestHash, err := git.GetLatestCommitHash(repoPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Latest hash: %s\n", latestHash)
		fmt.Printf("Length: %d\n", len(latestHash))
		if len(latestHash) != 40 {
			fmt.Printf("❌ WARNING: Hash length is %d, expected 40\n", len(latestHash))
		}
	}
	fmt.Println()

	// Test 2: Get commits since a specific hash
	fmt.Println("=== Test 2: GetCommitsSince (last 3 commits) ===")
	// Get the 4th commit hash to fetch last 3 commits
	cmd := exec.Command("git", "log", "--pretty=format:%H", "-n", "4")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error getting commits for test: %v\n", err)
		return
	}

	lines := []string{}
	start := 0
	for i := 0; i < len(output); i++ {
		if output[i] == '\n' {
			lines = append(lines, string(output[start:i]))
			start = i + 1
		}
	}
	if start < len(output) {
		lines = append(lines, string(output[start:]))
	}

	if len(lines) >= 4 {
		sinceHash := lines[3] // 4th commit
		fmt.Printf("Fetching commits since: %s\n", sinceHash)

		commits, err := git.GetCommitsSince(repoPath, sinceHash)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Found %d commits:\n", len(commits))
			for i, commit := range commits {
				fmt.Printf("%d. Hash: %s (len=%d)\n", i+1, commit.Hash, len(commit.Hash))
				fmt.Printf("   Author: %s\n", commit.Author)
				fmt.Printf("   Message: %s\n", commit.Message)
				fmt.Printf("   Timestamp: %s\n", commit.Timestamp)

				// Check for corruption pattern
				if len(commit.Hash) == 40 {
					firstHalf := commit.Hash[:20]
					secondHalf := commit.Hash[20:]
					if firstHalf == secondHalf {
						fmt.Printf("   ❌ WARNING: Hash has repeated pattern!\n")
					}
					// Check for e8e8e8 pattern
					if contains(commit.Hash[20:], "e8e8") {
						fmt.Printf("   ❌ WARNING: Hash contains e8e8 pattern in second half!\n")
					}
				} else if len(commit.Hash) != 40 {
					fmt.Printf("   ❌ WARNING: Hash length is %d, expected 40\n", len(commit.Hash))
				}
			}
		}
	} else {
		fmt.Println("Not enough commits in repository for test")
	}
	fmt.Println()

	// Test 3: Raw git log output
	fmt.Println("=== Test 3: Raw git log output (last commit) ===")
	cmd = exec.Command("git", "log", "--pretty=format:%H|%an|%s|%at", "-n", "1")
	cmd.Dir = repoPath
	output, err = cmd.Output()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Raw output: %s\n", string(output))
		fmt.Printf("Raw output length: %d\n", len(output))
		// Parse it
		parts := []string{}
		start := 0
		for i := 0; i < len(output); i++ {
			if output[i] == '|' {
				parts = append(parts, string(output[start:i]))
				start = i + 1
			}
		}
		if start < len(output) {
			parts = append(parts, string(output[start:]))
		}

		if len(parts) >= 1 {
			fmt.Printf("Parsed hash: %s\n", parts[0])
			fmt.Printf("Parsed hash length: %d\n", len(parts[0]))
			fmt.Printf("Parsed hash bytes: %v\n", []byte(parts[0]))
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
