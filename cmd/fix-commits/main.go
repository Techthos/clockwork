package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/techthos/clockwork/internal/db"
)

func main() {
	// Get database path
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get home directory: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(home, ".local", "clockwork", "default.db")

	// Open database
	store, err := db.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Get all projects
	projects, err := store.ListProjects()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list projects: %v\n", err)
		os.Exit(1)
	}

	fixedCount := 0

	for _, project := range projects {
		// Get all entries for this project
		entries, err := store.ListEntriesFiltered(project.ID, nil, nil, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list entries for %s: %v\n", project.Name, err)
			continue
		}

		for _, entry := range entries {
			if entry.CommitHash == "" {
				continue
			}

			// Validate commit hash in repository
			cmd := exec.Command("git", "cat-file", "-e", entry.CommitHash)
			cmd.Dir = project.GitRepoPath
			if err := cmd.Run(); err != nil {
				fmt.Printf("❌ Invalid hash in entry %s (project: %s, date: %s)\n",
					entry.ID, project.Name, entry.CreatedAt.Format("2006-01-02 15:04"))
				fmt.Printf("   Hash: %s\n", entry.CommitHash)

				// Get current HEAD to use as replacement
				cmd = exec.Command("git", "rev-parse", "HEAD")
				cmd.Dir = project.GitRepoPath
				output, err := cmd.Output()
				if err != nil {
					fmt.Printf("   ⚠️  Cannot get current HEAD, clearing hash instead\n")
					emptyHash := ""
					_, err = store.UpdateEntry(entry.ID, nil, nil, &emptyHash, nil, nil)
					if err != nil {
						fmt.Printf("   ❌ Failed to update: %v\n", err)
					} else {
						fmt.Printf("   ✓ Cleared commit hash\n")
						fixedCount++
					}
				} else {
					newHash := string(output)[:40]
					fmt.Printf("   → Updating to current HEAD: %s\n", newHash)
					_, err = store.UpdateEntry(entry.ID, nil, nil, &newHash, nil, nil)
					if err != nil {
						fmt.Printf("   ❌ Failed to update: %v\n", err)
					} else {
						fmt.Printf("   ✓ Fixed\n")
						fixedCount++
					}
				}
				fmt.Println()
			}
		}
	}

	if fixedCount == 0 {
		fmt.Println("✓ No invalid commit hashes found")
	} else {
		fmt.Printf("✓ Fixed %d invalid commit hash(es)\n", fixedCount)
	}
}
