package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// The form tools render the create/edit widgets and hydrate them with prefill
// values under the form's PrefillKey ("values"). Each is linked to its widget
// via _meta.ui.resourceUri, which is what makes the form discoverable as an
// MCP App; the form itself submits to the corresponding CRUD tool.

func (s *ClockworkServer) registerNewProjectForm() {
	tool := mcp.NewTool("new_project_form",
		mcp.WithDescription("Open an interactive form for creating a project. Submitting it calls create_project."),
	)
	linkWidget(&tool, s.widgets.projectCreate)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return s.formResult(s.widgets.projectCreate, "Fill in the new project form.",
			map[string]interface{}{}), nil
	})
}

func (s *ClockworkServer) registerEditProjectForm() {
	tool := mcp.NewTool("edit_project_form",
		mcp.WithDescription("Open an interactive form prefilled with a project's current values. Submitting it calls update_project."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Project ID")),
	)
	linkWidget(&tool, s.widgets.projectEdit)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := getRequiredString(request, "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		project, err := s.store.GetProject(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project not found: %v", err)), nil
		}

		return s.formResult(s.widgets.projectEdit, fmt.Sprintf("Editing project %q.", project.Name),
			projectValues(project)), nil
	})
}

func (s *ClockworkServer) registerNewEntryForm() {
	tool := mcp.NewTool("new_entry_form",
		mcp.WithDescription("Open an interactive form for a manual worklog entry on a project. Submitting it calls create_entry with manual=true."),
		mcp.WithString("project_id", mcp.Required(), mcp.Description("Project ID")),
	)
	linkWidget(&tool, s.widgets.entryCreate)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectID, err := getRequiredString(request, "project_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		project, err := s.store.GetProject(projectID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project not found: %v", err)), nil
		}

		return s.formResult(s.widgets.entryCreate, fmt.Sprintf("New manual entry for %q.", project.Name),
			map[string]interface{}{"project_id": project.ID}), nil
	})
}

func (s *ClockworkServer) registerEditEntryForm() {
	tool := mcp.NewTool("edit_entry_form",
		mcp.WithDescription("Open an interactive form prefilled with an entry's current values. Submitting it calls update_entry."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Entry ID")),
	)
	linkWidget(&tool, s.widgets.entryEdit)

	s.mcp.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := getRequiredString(request, "id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		entry, err := s.store.GetEntry(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("entry not found: %v", err)), nil
		}

		return s.formResult(s.widgets.entryEdit, fmt.Sprintf("Editing entry %s.", entry.ID),
			entryValues(entry)), nil
	})
}
