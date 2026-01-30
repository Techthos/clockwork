package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/techthos/clockwork/internal/db"
	"github.com/techthos/clockwork/internal/server"
	"github.com/techthos/clockwork/internal/tui"
)

func main() {
	// Check for TUI mode
	if len(os.Args) > 1 && os.Args[1] == "tui" {
		runTUI()
		return
	}

	// Default: Run MCP server
	runMCPServer()
}

func runTUI() {
	// Initialize database
	dbPath, err := getDBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve database path: %v\n", err)
		os.Exit(1)
	}

	store, err := db.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Create and run TUI application
	app := tui.New(store)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

func runMCPServer() {
	srv, err := server.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize server: %v\n", err)
		os.Exit(1)
	}
	defer srv.Close()

	if err := srv.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func getDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "clockwork", "default.db"), nil
}
