package server_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/techthos/gadget/uispec"

	"github.com/techthos/clockwork/internal/db"
	"github.com/techthos/clockwork/internal/server"
)

// newTestClient spins up the full server on a temporary database and connects
// an in-process MCP client, so tool calls exercise registration and
// (de)serialization without a transport.
func newTestClient(t *testing.T) *client.Client {
	t.Helper()

	store, err := db.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.New() error = %v", err)
	}

	srv := server.NewWithStore(store)
	t.Cleanup(func() {
		if err := srv.Close(); err != nil {
			t.Errorf("srv.Close() error = %v", err)
		}
	})

	c, err := client.NewInProcessClient(srv.MCP())
	if err != nil {
		t.Fatalf("NewInProcessClient() error = %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx := t.Context()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("client.Start() error = %v", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test-client", Version: "0.0.1"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatalf("client.Initialize() error = %v", err)
	}

	return c
}

func callTool(t *testing.T, c *client.Client, name string, args map[string]interface{}) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := c.CallTool(t.Context(), req)
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", name, err)
	}
	return res
}

// structured re-decodes a result's structuredContent, which is what carries
// widget data (rows / values / errors).
func structured(t *testing.T, res *mcp.CallToolResult) map[string]interface{} {
	t.Helper()
	if res.StructuredContent == nil {
		t.Fatal("result has no structuredContent")
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structuredContent: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal structuredContent: %v", err)
	}
	return out
}

// resourceURI reads a tool definition's _meta.ui.resourceUri — the field a
// host scans for to discover the tool's MCP App.
func resourceURI(tool mcp.Tool) string {
	if tool.Meta == nil {
		return ""
	}
	ui, ok := tool.Meta.AdditionalFields[uispec.MetaKey].(map[string]interface{})
	if !ok {
		return ""
	}
	uri, _ := ui["resourceUri"].(string)
	return uri
}

// embeddedHTML returns the text of the first embedded MCP Apps HTML resource
// (text/html;profile=mcp-app) in the result, or "" when none is present.
func embeddedHTML(res *mcp.CallToolResult) string {
	for _, content := range res.Content {
		embedded, ok := mcp.AsEmbeddedResource(content)
		if !ok {
			continue
		}
		if text, ok := embedded.Resource.(mcp.TextResourceContents); ok && text.MIMEType == uispec.MIMEType {
			return text.Text
		}
	}
	return ""
}

func TestToolsDeclareWidgetResourceURI(t *testing.T) {
	c := newTestClient(t)

	res, err := c.ListTools(t.Context(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	tools := map[string]mcp.Tool{}
	for _, tool := range res.Tools {
		tools[tool.Name] = tool
	}

	want := map[string]string{
		"list_projects":     "ui://clockwork/projects",
		"create_project":    "ui://clockwork/projects",
		"update_project":    "ui://clockwork/projects",
		"delete_project":    "ui://clockwork/projects",
		"list_entries":      "ui://clockwork/entries",
		"create_entry":      "ui://clockwork/entries",
		"update_entry":      "ui://clockwork/entries",
		"delete_entry":      "ui://clockwork/entries",
		"new_project_form":  "ui://clockwork/project-create-form",
		"edit_project_form": "ui://clockwork/project-edit-form",
		"new_entry_form":    "ui://clockwork/entry-create-form",
		"edit_entry_form":   "ui://clockwork/entry-edit-form",
		// No widget: plain JSON tools.
		"get_statistics":      "",
		"get_commit_baseline": "",
	}
	for name, wantURI := range want {
		tool, ok := tools[name]
		if !ok {
			t.Errorf("tool %s is not registered", name)
			continue
		}
		if got := resourceURI(tool); got != wantURI {
			t.Errorf("%s _meta.ui.resourceUri = %q, want %q", name, got, wantURI)
		}
	}
}

func TestWidgetResourcesAreServed(t *testing.T) {
	c := newTestClient(t)

	list, err := c.ListResources(t.Context(), mcp.ListResourcesRequest{})
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	registered := map[string]mcp.Resource{}
	for _, resource := range list.Resources {
		registered[resource.URI] = resource
	}

	for _, uri := range []string{
		"ui://clockwork/projects",
		"ui://clockwork/entries",
		"ui://clockwork/project-create-form",
		"ui://clockwork/project-edit-form",
		"ui://clockwork/entry-create-form",
		"ui://clockwork/entry-edit-form",
	} {
		t.Run(uri, func(t *testing.T) {
			resource, ok := registered[uri]
			if !ok {
				t.Fatalf("resource %s is not registered", uri)
			}
			if resource.MIMEType != uispec.MIMEType {
				t.Errorf("mimeType = %q, want %q", resource.MIMEType, uispec.MIMEType)
			}

			req := mcp.ReadResourceRequest{}
			req.Params.URI = uri
			read, err := c.ReadResource(t.Context(), req)
			if err != nil {
				t.Fatalf("ReadResource(%s) error = %v", uri, err)
			}
			if len(read.Contents) == 0 {
				t.Fatal("resource returned no contents")
			}
			text, ok := read.Contents[0].(mcp.TextResourceContents)
			if !ok {
				t.Fatalf("contents[0] is %T, want TextResourceContents", read.Contents[0])
			}
			if !strings.Contains(text.Text, "gadget-root") {
				t.Error("document does not look like a gadget widget")
			}
		})
	}
}

func TestProjectToolsCarryWidgetData(t *testing.T) {
	c := newTestClient(t)

	res := callTool(t, c, "create_project", map[string]interface{}{"name": "Widget Test"})
	if res.IsError {
		t.Fatalf("create_project failed: %+v", res.Content)
	}
	rows, ok := structured(t, res)["rows"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("create_project rows = %v, want the refreshed project list", structured(t, res)["rows"])
	}
	if embeddedHTML(res) == "" {
		t.Error("result carries no embedded widget document")
	}
	projectID, _ := rows[0].(map[string]interface{})["id"].(string)
	if projectID == "" {
		t.Fatal("row has no id")
	}

	// A validation failure is a tool-level error that also hands the form its
	// inline field errors and the values the user typed.
	res = callTool(t, c, "create_project", map[string]interface{}{"name": "Bad", "source_type": "bogus"})
	if !res.IsError {
		t.Fatal("create_project with an unknown source_type should be an error result")
	}
	content := structured(t, res)
	errs, ok := content["errors"].(map[string]interface{})
	if !ok || errs["source_type"] == nil {
		t.Errorf("errors = %v, want an inline source_type error", content["errors"])
	}
	values, ok := content["values"].(map[string]interface{})
	if !ok || values["name"] != "Bad" {
		t.Errorf("values = %v, want the submitted fields", content["values"])
	}

	// The edit form hydrates from the stored project.
	res = callTool(t, c, "edit_project_form", map[string]interface{}{"id": projectID})
	if res.IsError {
		t.Fatalf("edit_project_form failed: %+v", res.Content)
	}
	values, ok = structured(t, res)["values"].(map[string]interface{})
	if !ok || values["id"] != projectID || values["name"] != "Widget Test" {
		t.Errorf("values = %v, want the stored project prefill", structured(t, res)["values"])
	}

	res = callTool(t, c, "delete_project", map[string]interface{}{"id": projectID})
	if res.IsError {
		t.Fatalf("delete_project failed: %+v", res.Content)
	}
	if rows, ok := structured(t, res)["rows"].([]interface{}); !ok || len(rows) != 0 {
		t.Errorf("delete_project rows = %v, want the refreshed (empty) list", structured(t, res)["rows"])
	}
}

func TestEntryToolsCarryWidgetData(t *testing.T) {
	c := newTestClient(t)

	createRes := callTool(t, c, "create_project", map[string]interface{}{"name": "Entries"})
	if createRes.IsError {
		t.Fatalf("create_project failed: %+v", createRes.Content)
	}
	rows := structured(t, createRes)["rows"].([]interface{})
	projectID := rows[0].(map[string]interface{})["id"].(string)

	res := callTool(t, c, "create_entry", map[string]interface{}{
		"project_id": projectID,
		"manual":     true,
		"duration":   "1h 30m",
		"message":    "widget test entry",
	})
	if res.IsError {
		t.Fatalf("create_entry failed: %+v", res.Content)
	}
	entryRows, ok := structured(t, res)["rows"].([]interface{})
	if !ok || len(entryRows) != 1 {
		t.Fatalf("create_entry rows = %v, want the refreshed entry list", structured(t, res)["rows"])
	}
	entryID := entryRows[0].(map[string]interface{})["id"].(string)

	res = callTool(t, c, "list_entries", map[string]interface{}{"project_id": projectID})
	if res.IsError {
		t.Fatalf("list_entries failed: %+v", res.Content)
	}
	if rows, ok := structured(t, res)["rows"].([]interface{}); !ok || len(rows) != 1 {
		t.Errorf("list_entries rows = %v, want one entry", structured(t, res)["rows"])
	}
	if embeddedHTML(res) == "" {
		t.Error("list_entries carries no embedded widget document")
	}

	// Invalid manual duration returns inline form errors alongside the values.
	res = callTool(t, c, "create_entry", map[string]interface{}{
		"project_id": projectID,
		"manual":     true,
		"duration":   "garbage",
	})
	if !res.IsError {
		t.Fatal("create_entry with an invalid duration should be an error result")
	}
	content := structured(t, res)
	if errs, ok := content["errors"].(map[string]interface{}); !ok || errs["duration"] == nil {
		t.Errorf("errors = %v, want an inline duration error", content["errors"])
	}
	if values, ok := content["values"].(map[string]interface{}); !ok || values["project_id"] != projectID {
		t.Errorf("values = %v, want the project_id preserved for resubmit", content["values"])
	}

	// The entry edit form hydrates from the stored entry.
	res = callTool(t, c, "edit_entry_form", map[string]interface{}{"id": entryID})
	if res.IsError {
		t.Fatalf("edit_entry_form failed: %+v", res.Content)
	}
	values, ok := structured(t, res)["values"].(map[string]interface{})
	if !ok || values["id"] != entryID || values["duration_string"] != "90m" {
		t.Errorf("values = %v, want the stored entry prefill", structured(t, res)["values"])
	}
}
