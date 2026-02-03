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
	fmt.Printf("Database path: %s\n", dbPath)

	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Database does not exist at %s\n", dbPath)
		os.Exit(1)
	}

	// Open database
	store, err := db.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// List all projects
	projects, err := store.ListProjects()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list projects: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nFound %d project(s):\n\n", len(projects))

	for i, project := range projects {
		fmt.Printf("=== Project %d ===\n", i+1)
		fmt.Printf("  Name: %s\n", project.Name)
		fmt.Printf("  ID: %s\n", project.ID)
		fmt.Printf("  Git Repo Path: %s\n", project.GitRepoPath)

		// Check if path exists
		absPath, err := filepath.Abs(project.GitRepoPath)
		if err != nil {
			fmt.Printf("  ❌ Path Error: %v\n", err)
			continue
		}
		fmt.Printf("  Absolute Path: %s\n", absPath)

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			fmt.Printf("  ❌ Status: Path does not exist\n")
		} else if err != nil {
			fmt.Printf("  ❌ Status: Cannot access path (%v)\n", err)
		} else {
			fmt.Printf("  ✓ Status: Path exists\n")

			// Check if it's a git repository
			cmd := exec.Command("git", "rev-parse", "--git-dir")
			cmd.Dir = absPath
			output, err := cmd.Output()
			if err != nil {
				fmt.Printf("  ❌ Git Status: Not a git repository (exit %v)\n", err)
			} else {
				gitDir := string(output)
				fmt.Printf("  ✓ Git Status: Valid repository (%s)\n", gitDir[:len(gitDir)-1])

				// Get HEAD commit
				cmd = exec.Command("git", "rev-parse", "HEAD")
				cmd.Dir = absPath
				output, err = cmd.Output()
				if err != nil {
					fmt.Printf("  ❌ HEAD: Cannot resolve HEAD (%v)\n", err)
				} else {
					headHash := string(output)[:40]
					fmt.Printf("  ✓ HEAD: %s\n", headHash)
				}
			}
		}

		// Get last entry for this project
		lastEntry, err := store.GetLastEntry(project.ID)
		if err != nil {
			fmt.Printf("  ❌ Last Entry Error: %v\n", err)
		} else if lastEntry != nil {
			fmt.Printf("  Last Entry: %s\n", lastEntry.CreatedAt.Format("2006-01-02 15:04"))
			fmt.Printf("  Last Commit Hash: %s\n", lastEntry.CommitHash)

			// Validate last commit hash if we have git access
			if lastEntry.CommitHash != "" {
				cmd := exec.Command("git", "cat-file", "-e", lastEntry.CommitHash)
				cmd.Dir = absPath
				if err := cmd.Run(); err != nil {
					fmt.Printf("  ❌ Last Commit Status: Hash does not exist in repository\n")
					fmt.Printf("     This is likely the cause of exit status 128!\n")
					fmt.Printf("     The repository may have been reset or force-pushed.\n")
				} else {
					fmt.Printf("  ✓ Last Commit Status: Hash exists in repository\n")
				}
			}
		} else {
			fmt.Printf("  Last Entry: None (first entry will work)\n")
		}

		fmt.Println()
	}

	fmt.Println("\nDiagnostic complete.")
}
