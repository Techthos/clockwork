package server

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/techthos/gadget"

	"github.com/techthos/clockwork/internal/models"
	"github.com/techthos/clockwork/internal/source"
)

// This file implements the interactive UI side of the CRUD tools as MCP Apps
// (extension io.modelcontextprotocol/ui, spec 2026-01-26), the way the spec
// intends:
//
//   - every widget is registered ONCE as a stable ui:// template resource
//     (`registerWidgets`), rendered at construction and served verbatim by
//     resources/read. The document carries no data.
//   - every tool that renders a widget declares it in its definition via
//     `_meta.ui.resourceUri` (`linkWidget`). This — not anything in the tool
//     result — is how a host discovers the app.
//   - data reaches the widget through the tool result's structuredContent,
//     keyed by the widget's RowsKey ("rows") / PrefillKey ("values") /
//     ErrorsKey ("errors").
//   - actions and submits target the normal model-visible tools; the gadget
//     runtime speaks the App Bridge (tools/call, ui/open-link,
//     ui/notifications/size-changed) on the widget's behalf.
//
// As a supplement for hosts that render result-embedded widgets, each result
// also carries the registered document (same stable URI, same bytes) as an
// embedded resource. It is a fallback, never the discovery mechanism, and the
// text + structuredContent result always stands alone: widget failures are
// logged to stderr and never fail a tool.

// Stable widget URIs — one per widget kind. Both the registered resource and
// the linking tool's _meta point here, so these must never vary per render.
const (
	uriProjectsTable = "ui://clockwork/projects"
	uriEntriesTable  = "ui://clockwork/entries"
	uriProjectCreate = "ui://clockwork/project-create-form"
	uriProjectEdit   = "ui://clockwork/project-edit-form"
	uriEntryCreate   = "ui://clockwork/entry-create-form"
	uriEntryEdit     = "ui://clockwork/entry-edit-form"
)

// widgets holds the registered widget templates plus their rendered
// documents, so linking a tool and embedding the fallback document need no
// re-render.
type widgets struct {
	projects      *gadget.Table
	entries       *gadget.Table
	projectCreate *gadget.Form
	projectEdit   *gadget.Form
	entryCreate   *gadget.Form
	entryEdit     *gadget.Form

	docs map[string]string // URI -> rendered document
}

// registerWidgets builds every widget, registers it as a ui:// resource and
// caches its document. A widget that fails to render is skipped: the tools
// keep working, only their UI is missing.
func (s *ClockworkServer) registerWidgets() {
	s.widgets = &widgets{
		projects:      projectsTable(),
		entries:       entriesTable(),
		projectCreate: projectCreateForm(),
		projectEdit:   projectEditForm(),
		entryCreate:   entryCreateForm(),
		entryEdit:     entryEditForm(),
		docs:          map[string]string{},
	}

	for _, w := range []gadget.Widget{
		s.widgets.projects, s.widgets.entries,
		s.widgets.projectCreate, s.widgets.projectEdit,
		s.widgets.entryCreate, s.widgets.entryEdit,
	} {
		s.registerWidget(w)
	}
}

// registerWidget renders w once and serves it from memory at its stable URI.
func (s *ClockworkServer) registerWidget(w gadget.Widget) {
	doc, err := w.Document()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: widget render failed: %v\n", err)
		return
	}
	d := w.Descriptor()
	s.widgets.docs[d.URI] = doc

	resource := mcp.NewResource(d.URI, d.Title, mcp.WithMIMEType(d.MIMEType))
	s.mcp.AddResource(resource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{mcp.TextResourceContents{
			URI:      d.URI,
			MIMEType: d.MIMEType,
			Text:     doc,
		}}, nil
	})
}

// linkWidget declares in the tool definition which widget renders this tool —
// `_meta.ui.resourceUri`. A tool without it is invisible as an MCP App no
// matter what its result contains.
func linkWidget(tool *mcp.Tool, w gadget.Widget) {
	tool.Meta = mcp.NewMetaFromMap(w.ToolMeta())
}

// embed appends the registered document of the widget this result hydrates,
// for hosts that render result-embedded widgets. Same URI and bytes as the
// registered template, so nothing can go stale.
func (s *ClockworkServer) embed(res *mcp.CallToolResult, w gadget.Widget) *mcp.CallToolResult {
	if res == nil || w == nil || s.widgets == nil {
		return res
	}
	d := w.Descriptor()
	doc, ok := s.widgets.docs[d.URI]
	if !ok {
		return res
	}
	res.Content = append(res.Content, mcp.NewEmbeddedResource(mcp.TextResourceContents{
		URI:      d.URI,
		MIMEType: d.MIMEType,
		Text:     doc,
	}))
	return res
}

// rowsOrLog converts a typed slice into widget rows, logging (not failing) on
// marshal errors so the widget degrades to its empty state. The result is
// never nil: an empty list must reach the widget as [] so a table repaints
// after its last row is deleted instead of keeping the stale rows.
func rowsOrLog(slice interface{}) []map[string]interface{} {
	rows, err := gadget.RowsOf(slice)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: widget rows conversion failed: %v\n", err)
		return []map[string]interface{}{}
	}
	if rows == nil {
		return []map[string]interface{}{}
	}
	return rows
}

// projectsTable is the projects list widget. It hydrates itself on a fresh
// mount via LoadTool, and repaints from any tool result carrying "rows".
// Delete is a row action with inline confirmation (the sandboxed iframe has
// no native confirm()).
func projectsTable() *gadget.Table {
	return &gadget.Table{
		URI:      uriProjectsTable,
		Title:    "Projects",
		LoadTool: "list_projects",
		Columns: []gadget.Column{
			gadget.Text("name", "Name"),
			gadget.Badge("source_type", "Source", map[string]gadget.BadgeVariant{
				string(source.None):  gadget.BadgeNeutral,
				string(source.Local): gadget.BadgeInfo,
				string(source.MCP):   gadget.BadgeSuccess,
			}),
			gadget.Text("git_repo_path", "Repo path"),
			gadget.Text("repository", "Repository"),
			gadget.Date("created_at", "Created", "date"),
			gadget.ActionsColumn(
				gadget.Action{
					Label: "Edit",
					Tool:  "edit_project_form",
					Args:  map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
				},
				gadget.Action{
					Label:   "Delete",
					Tool:    "delete_project",
					Args:    map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
					Confirm: "Delete this project and all its entries? This cannot be undone.",
					Variant: gadget.VariantDanger,
				},
			),
		},
		Filterable: true,
		PageSize:   10,
		Empty:      gadget.EmptyState{Title: "No projects", Body: "Create one with create_project."},
	}
}

// entriesTable is the worklog entries widget. Its LoadTool is unscoped, so a
// remounted table shows all entries until the next scoped result arrives.
func entriesTable() *gadget.Table {
	return &gadget.Table{
		URI:      uriEntriesTable,
		Title:    "Worklog entries",
		LoadTool: "list_entries",
		Columns: []gadget.Column{
			gadget.Text("message", "Message"),
			gadget.Number("duration", "Minutes", "int"),
			gadget.Badge("invoiced", "Invoiced", map[string]gadget.BadgeVariant{
				"true":  gadget.BadgeSuccess,
				"false": gadget.BadgeNeutral,
			}),
			gadget.Text("project_id", "Project"),
			gadget.Date("created_at", "Created", "datetime"),
			gadget.ActionsColumn(
				gadget.Action{
					Label: "Edit",
					Tool:  "edit_entry_form",
					Args:  map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
				},
				gadget.Action{
					Label:   "Delete",
					Tool:    "delete_entry",
					Args:    map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
					Confirm: "Delete this entry? This cannot be undone.",
					Variant: gadget.VariantDanger,
				},
			),
		},
		Filterable:  true,
		PageSize:    10,
		DefaultSort: &gadget.SortSpec{Key: "created_at", Desc: true},
		Empty:       gadget.EmptyState{Title: "No entries", Body: "Create one with create_entry."},
	}
}

// sourceOptions lists the commit lookup methods for the project forms.
func sourceOptions() []gadget.Option {
	options := make([]gadget.Option, 0, len(source.All()))
	for _, t := range source.All() {
		options = append(options, gadget.Option{Value: t.String(), Label: fmt.Sprintf("%s - %s", t, source.Describe(t))})
	}
	return options
}

// projectFields are the editable project fields, shared by both project forms.
func projectFields() []gadget.Field {
	return []gadget.Field{
		{Name: "name", Label: "Name", Required: true},
		{Name: "source_type", Label: "Source", Type: gadget.FSelect, Options: sourceOptions(),
			Description: "How commits are looked up for this project."},
		{Name: "git_repo_path", Label: "Git repository path",
			Description: "Absolute path on this filesystem. Only for source 'local'."},
		{Name: "repository", Label: "Repository identifier",
			Description: "e.g. 'owner/name' or a clone URL. Only for source 'mcp'."},
	}
}

// projectCreateForm submits to create_project. Its LoadTool is the same tool
// that renders it, so a remount comes back blank rather than stale.
func projectCreateForm() *gadget.Form {
	return &gadget.Form{
		URI:      uriProjectCreate,
		Title:    "New project",
		LoadTool: "new_project_form",
		Fields:   projectFields(),
		Submit: gadget.SubmitSpec{
			Tool:           "create_project",
			Label:          "Create",
			SuccessMessage: "Project created.",
		},
	}
}

// projectEditForm submits to update_project. The project id travels in a
// hidden field prefilled from the tool result, because update_project needs it
// as an argument.
//
// No LoadTool: hydration is id-scoped and LoadArgs are fixed at registration,
// so an argument-free reload would wipe the prefill instead of refreshing it.
// Updates are presence-based and text fields always submit, which is why every
// field is prefilled — a blank field would clear the stored value.
func projectEditForm() *gadget.Form {
	fields := append([]gadget.Field{{Name: "id", Type: gadget.FHidden}}, projectFields()...)
	return &gadget.Form{
		URI:    uriProjectEdit,
		Title:  "Edit project",
		Fields: fields,
		Submit: gadget.SubmitSpec{
			Tool:           "update_project",
			Label:          "Save",
			SuccessMessage: "Project saved.",
		},
	}
}

// entryCreateForm submits manual entries: create_entry with manual=true fixed,
// so only the fields a user actually fills are exposed. The project id is a
// hidden prefilled field (see projectEditForm for why there is no LoadTool).
func entryCreateForm() *gadget.Form {
	return &gadget.Form{
		URI:   uriEntryCreate,
		Title: "New manual entry",
		Fields: []gadget.Field{
			{Name: "project_id", Type: gadget.FHidden},
			{Name: "duration", Label: "Duration", Required: true, Placeholder: "1h 30m",
				Description: "Duration in the form '1h 30m' or '90m'."},
			{Name: "message", Label: "Message", Type: gadget.FTextarea},
			{Name: "invoiced", Label: "Invoiced", Type: gadget.FCheckbox},
		},
		Submit: gadget.SubmitSpec{
			Tool:           "create_entry",
			Label:          "Create",
			StaticArgs:     map[string]interface{}{"manual": true},
			SuccessMessage: "Entry created.",
		},
	}
}

// entryEditForm submits to update_entry with the entry id in a hidden field.
func entryEditForm() *gadget.Form {
	return &gadget.Form{
		URI:   uriEntryEdit,
		Title: "Edit entry",
		Fields: []gadget.Field{
			{Name: "id", Type: gadget.FHidden},
			{Name: "duration_string", Label: "Duration", Placeholder: "1h 30m",
				Description: "Duration in the form '1h 30m' or '90m'."},
			{Name: "message", Label: "Message", Type: gadget.FTextarea},
			{Name: "invoiced", Label: "Invoiced", Type: gadget.FCheckbox},
		},
		Submit: gadget.SubmitSpec{
			Tool:           "update_entry",
			Label:          "Save",
			SuccessMessage: "Entry updated.",
		},
	}
}

// projectValues renders a project as form prefill.
func projectValues(project *models.Project) map[string]interface{} {
	if project == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":            project.ID,
		"name":          project.Name,
		"source_type":   source.Resolve(project).String(),
		"git_repo_path": project.GitRepoPath,
		"repository":    project.Repository,
	}
}

// entryValues renders an entry as form prefill.
func entryValues(entry *models.Entry) map[string]interface{} {
	if entry == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":              entry.ID,
		"duration_string": fmt.Sprintf("%dm", entry.Duration),
		"message":         entry.Message,
		"invoiced":        entry.Invoiced,
	}
}

// rowsResult builds a table result: the rows under the table's rows key, a
// one-line status for the host banner, and any extra keys the model needs.
func (s *ClockworkServer) rowsResult(table *gadget.Table, status string, rows []map[string]interface{}, extra map[string]interface{}) *mcp.CallToolResult {
	payload := map[string]interface{}{"rows": rows}
	for key, val := range extra {
		payload[key] = val
	}
	return s.embed(mcp.NewToolResultStructured(payload, status), table)
}

// projectsResult returns the refreshed project list. Every mutating project
// tool returns one of these, otherwise the visible table goes stale after a
// successful call.
func (s *ClockworkServer) projectsResult(status string, extra map[string]interface{}) *mcp.CallToolResult {
	projects, err := s.store.ListProjects()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load projects for widget: %v\n", err)
	}
	return s.rowsResult(s.widgets.projects, status, rowsOrLog(projects), extra)
}

// entriesResult is the entries counterpart of projectsResult, scoped to one
// project when projectID is set.
func (s *ClockworkServer) entriesResult(status, projectID string, extra map[string]interface{}) *mcp.CallToolResult {
	entries, err := s.store.ListEntriesFiltered(projectID, nil, nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load entries for widget: %v\n", err)
	}
	return s.rowsResult(s.widgets.entries, status, rowsOrLog(entries), extra)
}

// formResult renders a form with prefill values.
func (s *ClockworkServer) formResult(form *gadget.Form, status string, values map[string]interface{}) *mcp.CallToolResult {
	return s.embed(mcp.NewToolResultStructured(map[string]interface{}{"values": values}, status), form)
}

// formError fails a tool at the user level while handing the submitting form
// its inline field errors: "errors" marks the submit failed, "values" keeps
// what the user typed.
func (s *ClockworkServer) formError(form *gadget.Form, message string, values map[string]interface{}, errs map[string]string) *mcp.CallToolResult {
	res := mcp.NewToolResultError(message)
	res.StructuredContent = map[string]interface{}{
		"values": values,
		"errors": errs,
	}
	return s.embed(res, form)
}
