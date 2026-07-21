package source

import (
	"testing"

	"github.com/techthos/clockwork/internal/models"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input   string
		want    Type
		wantErr bool
	}{
		{"none", None, false},
		{"local", Local, false},
		{"mcp", MCP, false},
		{"MCP", MCP, false},
		{"  Local  ", Local, false},
		{"git", Local, false}, // legacy alias
		{"", "", true},
		{"api", "", true},
		{"nonsense", "", true},
	}

	for _, tt := range tests {
		got, err := Parse(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Parse(%q): expected error, got %q", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveInfersLegacyProjects(t *testing.T) {
	tests := []struct {
		name    string
		project *models.Project
		want    Type
	}{
		{"nil project", nil, None},
		{"explicit type wins", &models.Project{SourceType: "mcp", Repository: "owner/name"}, MCP},
		{"legacy row with path", &models.Project{GitRepoPath: "/repo"}, Local},
		{"legacy row without path", &models.Project{}, None},
		{"unparseable type falls back to locator", &models.Project{SourceType: "bogus", GitRepoPath: "/repo"}, Local},
	}

	for _, tt := range tests {
		if got := Resolve(tt.project); got != tt.want {
			t.Errorf("%s: Resolve() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestLocator(t *testing.T) {
	tests := []struct {
		name    string
		project *models.Project
		want    string
	}{
		{"local uses path", &models.Project{SourceType: "local", GitRepoPath: "/repo"}, "/repo"},
		{"mcp uses repository", &models.Project{SourceType: "mcp", Repository: "owner/name"}, "owner/name"},
		{"none has no locator", &models.Project{SourceType: "none"}, ""},
	}

	for _, tt := range tests {
		if got := Locator(tt.project); got != tt.want {
			t.Errorf("%s: Locator() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		typ         Type
		gitRepoPath string
		repository  string
		wantErr     bool
	}{
		{"none with nothing", None, "", "", false},
		{"none with a path", None, "/repo", "", true},
		{"none with a repository", None, "", "owner/name", true},
		{"local with a path", Local, "/repo", "", false},
		{"local without a path", Local, "", "", true},
		{"local with a repository too", Local, "/repo", "owner/name", true},
		{"mcp with a repository", MCP, "", "owner/name", false},
		{"mcp without a repository", MCP, "", "", true},
		{"mcp with a path too", MCP, "/repo", "owner/name", true},
		{"unknown type", Type("api"), "", "", true},
	}

	for _, tt := range tests {
		err := Validate(tt.typ, tt.gitRepoPath, tt.repository)
		if tt.wantErr && err == nil {
			t.Errorf("%s: expected error, got nil", tt.name)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("%s: unexpected error: %v", tt.name, err)
		}
	}
}

func TestInfer(t *testing.T) {
	if got := Infer("", ""); got != None {
		t.Errorf("Infer with nothing = %q, want %q", got, None)
	}
	if got := Infer("/repo", ""); got != Local {
		t.Errorf("Infer with path = %q, want %q", got, Local)
	}
	if got := Infer("", "owner/name"); got != MCP {
		t.Errorf("Infer with repository = %q, want %q", got, MCP)
	}
}

func TestSuppliesCommits(t *testing.T) {
	if !MCP.SuppliesCommits() {
		t.Error("MCP commits should be supplied by the caller")
	}
	if Local.SuppliesCommits() || None.SuppliesCommits() {
		t.Error("only MCP commits should be supplied by the caller")
	}
}
