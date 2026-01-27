package main

import (
	"fmt"
	"os"

	"github.com/techthos/clockwork/internal/server"
)

func main() {
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
