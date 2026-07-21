// Package source describes where a project's commits come from.
//
// Clockwork never reaches out over the network itself. A project either has no
// repository at all, has a git repository on the local filesystem that
// clockwork reads with the git CLI, or has a repository that only the calling
// client can see — in which case the client supplies the commits.
package source

import (
	"fmt"
	"strings"

	"github.com/techthos/clockwork/internal/models"
)

// Type identifies a project's commit lookup method.
type Type string

const (
	// None means the project has no repository. Only manual entries are possible.
	None Type = "none"

	// Local means commits live in a git repository on this filesystem and are
	// read by running the git CLI against GitRepoPath.
	Local Type = "local"

	// MCP means clockwork does not look commits up at all. The caller — an AI
	// client with access to a repository MCP server, a hosted API, or anything
	// else — fetches them and passes them in when creating an entry.
	MCP Type = "mcp"
)

// All returns every supported lookup method, in presentation order.
func All() []Type { return []Type{None, Local, MCP} }

// Parse converts a user-supplied string into a Type.
func Parse(s string) (Type, error) {
	switch Type(strings.ToLower(strings.TrimSpace(s))) {
	case None:
		return None, nil
	case Local, "git":
		// "git" is accepted as an alias for the original file-based behaviour.
		return Local, nil
	case MCP:
		return MCP, nil
	}
	return "", fmt.Errorf("unknown source type %q (expected one of: none, local, mcp)", s)
}

func (t Type) String() string { return string(t) }

// SuppliesCommits reports whether the caller is responsible for providing
// commits rather than clockwork reading them.
func (t Type) SuppliesCommits() bool { return t == MCP }

// HasRepository reports whether the project is backed by a repository at all.
func (t Type) HasRepository() bool { return t == Local || t == MCP }

// Describe returns a one-line explanation, used in tool schemas and the TUI.
func Describe(t Type) string {
	switch t {
	case None:
		return "no repository - manual entries only"
	case Local:
		return "git repository on this filesystem, read with the git CLI"
	case MCP:
		return "commits are supplied by the calling client (e.g. an AI using a repository MCP server or API)"
	}
	return string(t)
}

// Resolve returns a project's effective lookup method, inferring one for rows
// written before source_type existed.
func Resolve(p *models.Project) Type {
	if p == nil {
		return None
	}
	if t, err := Parse(p.SourceType); err == nil {
		return t
	}
	if p.GitRepoPath != "" {
		return Local
	}
	return None
}

// Locator returns the identifier of the project's repository: a filesystem
// path for Local, a client-meaningful identifier for MCP, empty for None.
func Locator(p *models.Project) string {
	switch Resolve(p) {
	case Local:
		return p.GitRepoPath
	case MCP:
		return p.Repository
	}
	return ""
}

// Validate checks that the repository fields are consistent with the type.
// It deliberately performs no filesystem or network access — that belongs to
// the layer that knows whether the caller can be prompted about it.
func Validate(t Type, gitRepoPath, repository string) error {
	switch t {
	case None:
		if gitRepoPath != "" || repository != "" {
			return fmt.Errorf("source type %q must not have a repository configured", t)
		}
	case Local:
		if gitRepoPath == "" {
			return fmt.Errorf("source type %q requires git_repo_path", t)
		}
		if repository != "" {
			return fmt.Errorf("source type %q uses git_repo_path, not repository", t)
		}
	case MCP:
		if repository == "" {
			return fmt.Errorf("source type %q requires repository: the identifier the client uses to look the project up, e.g. 'owner/name' or a clone URL", t)
		}
		if gitRepoPath != "" {
			return fmt.Errorf("source type %q uses repository, not git_repo_path", t)
		}
	default:
		return fmt.Errorf("unknown source type %q", t)
	}
	return nil
}

// Infer picks a lookup method from whichever repository field was supplied.
// It keeps callers that predate source_type working unchanged.
func Infer(gitRepoPath, repository string) Type {
	switch {
	case repository != "":
		return MCP
	case gitRepoPath != "":
		return Local
	default:
		return None
	}
}
