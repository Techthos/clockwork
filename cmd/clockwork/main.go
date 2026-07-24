package main

import (
	"fmt"
	"os"
	"strings"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/techthos/clockwork/internal/db"
	"github.com/techthos/clockwork/internal/server"
	"github.com/techthos/clockwork/internal/tui"
)

const usage = `clockwork - git-driven time tracking

Usage:
  clockwork [--db <path>]        Run the TUI (default)
  clockwork mcp [--db <path>]    Run the MCP server on stdio

Database path precedence: --db, then $CLOCKWORK_DB, then ~/.local/clockwork/default.db.
`

func main() {
	args, dbFlag, err := extractDBFlag(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n\n%s", err, usage)
		os.Exit(2)
	}

	if len(args) > 0 {
		switch args[0] {
		case "help", "-h", "--help":
			fmt.Print(usage)
			return
		case "mcp":
		default:
			fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", args[0], usage)
			os.Exit(2)
		}
	}

	store, err := openStore(dbFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if len(args) > 0 && args[0] == "mcp" {
		runMCPServer(store)
		return
	}
	runTUI(store)
}

// extractDBFlag pulls --db/-db (in either "--db path" or "--db=path" form) out
// of the argument list and returns the rest. An explicit flag outranks the
// environment variable; resolving the default stays in internal/db.
func extractDBFlag(argv []string) (rest []string, dbPath string, err error) {
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		name, value, hasValue := strings.Cut(arg, "=")
		if name != "--db" && name != "-db" {
			rest = append(rest, arg)
			continue
		}
		if !hasValue {
			if i+1 >= len(argv) {
				return nil, "", fmt.Errorf("flag %s needs a path", name)
			}
			i++
			value = argv[i]
		}
		if strings.TrimSpace(value) == "" {
			return nil, "", fmt.Errorf("flag %s needs a path", name)
		}
		dbPath = value
	}
	return rest, dbPath, nil
}

// openStore resolves the database path (flag first, then the resolver) and
// opens the store. Both modes go through it, so they can never end up on
// different files.
func openStore(dbFlag string) (*db.Store, error) {
	dbPath := dbFlag
	if dbPath == "" {
		resolved, err := db.DefaultPath()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve database path: %w", err)
		}
		dbPath = resolved
	}

	store, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	return store, nil
}

func runTUI(store *db.Store) {
	app := tui.New(store)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

func runMCPServer(store *db.Store) {
	srv := server.NewWithStore(store)

	// stdio transport: stdout is the protocol channel, all logging goes to stderr.
	if err := mcpserver.ServeStdio(srv.MCP()); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
