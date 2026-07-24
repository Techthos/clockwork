package server_test

import (
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// entryRowMessages extracts the message column of a widget row set.
func entryRowMessages(t *testing.T, res *mcp.CallToolResult) []string {
	t.Helper()
	rows, ok := structured(t, res)["rows"].([]interface{})
	if !ok {
		t.Fatalf("result carries no rows: %+v", res.StructuredContent)
	}
	messages := make([]string, 0, len(rows))
	for _, row := range rows {
		obj, ok := row.(map[string]interface{})
		if !ok {
			t.Fatalf("row is not an object: %v", row)
		}
		msg, _ := obj["message"].(string)
		messages = append(messages, msg)
	}
	return messages
}

// pageMeta reads the pagination keys the list tool returns alongside the rows.
func pageMeta(t *testing.T, res *mcp.CallToolResult, key string) float64 {
	t.Helper()
	val, ok := structured(t, res)[key].(float64)
	if !ok {
		t.Fatalf("result has no numeric %q: %+v", key, res.StructuredContent)
	}
	return val
}

func TestListEntriesPaginates(t *testing.T) {
	c := newTestClient(t)

	createRes := callTool(t, c, "create_project", map[string]interface{}{"name": "Paging"})
	if createRes.IsError {
		t.Fatalf("create_project failed: %+v", createRes.Content)
	}
	projectID := structured(t, createRes)["rows"].([]interface{})[0].(map[string]interface{})["id"].(string)

	// Five entries, one per day, so creation order is unambiguous.
	for i := 1; i <= 5; i++ {
		res := callTool(t, c, "create_entry", map[string]interface{}{
			"project_id": projectID,
			"manual":     true,
			"duration":   fmt.Sprintf("%dm", 30*i),
			"message":    fmt.Sprintf("entry %d", i),
			"created_at": fmt.Sprintf("2026-01-0%dT09:00:00Z", i),
		})
		if res.IsError {
			t.Fatalf("create_entry %d failed: %+v", i, res.Content)
		}
	}

	res := callTool(t, c, "list_entries", map[string]interface{}{
		"project_id": projectID,
		"page_size":  2,
	})
	if res.IsError {
		t.Fatalf("list_entries failed: %+v", res.Content)
	}
	if got, want := entryRowMessages(t, res), []string{"entry 5", "entry 4"}; !equalStrings(got, want) {
		t.Errorf("page 1 messages = %v, want %v (newest first)", got, want)
	}
	if got := pageMeta(t, res, "total"); got != 5 {
		t.Errorf("total = %v, want 5", got)
	}
	if got := pageMeta(t, res, "total_pages"); got != 3 {
		t.Errorf("total_pages = %v, want 3", got)
	}
	if has, ok := structured(t, res)["has_more"].(bool); !ok || !has {
		t.Errorf("has_more = %v, want true", structured(t, res)["has_more"])
	}

	res = callTool(t, c, "list_entries", map[string]interface{}{
		"project_id": projectID,
		"page":       3,
		"page_size":  2,
	})
	if res.IsError {
		t.Fatalf("list_entries page 3 failed: %+v", res.Content)
	}
	if got, want := entryRowMessages(t, res), []string{"entry 1"}; !equalStrings(got, want) {
		t.Errorf("page 3 messages = %v, want %v", got, want)
	}
	if has, ok := structured(t, res)["has_more"].(bool); !ok || has {
		t.Errorf("has_more on the last page = %v, want false", structured(t, res)["has_more"])
	}
}

func TestListEntriesSearchAndOrder(t *testing.T) {
	c := newTestClient(t)

	createRes := callTool(t, c, "create_project", map[string]interface{}{"name": "Searchable"})
	projectID := structured(t, createRes)["rows"].([]interface{})[0].(map[string]interface{})["id"].(string)

	for i, msg := range []string{"sprint planning", "invoice run", "code review"} {
		res := callTool(t, c, "create_entry", map[string]interface{}{
			"project_id": projectID,
			"manual":     true,
			"duration":   fmt.Sprintf("%dm", 30*(i+1)),
			"message":    msg,
			"created_at": fmt.Sprintf("2026-02-0%dT09:00:00Z", i+1),
		})
		if res.IsError {
			t.Fatalf("create_entry %q failed: %+v", msg, res.Content)
		}
	}

	res := callTool(t, c, "list_entries", map[string]interface{}{"search": "INVOICE"})
	if res.IsError {
		t.Fatalf("list_entries with search failed: %+v", res.Content)
	}
	if got, want := entryRowMessages(t, res), []string{"invoice run"}; !equalStrings(got, want) {
		t.Errorf("search messages = %v, want %v", got, want)
	}

	// The project name is searchable too, even though entries store only its id.
	res = callTool(t, c, "list_entries", map[string]interface{}{"search": "searchable"})
	if got := len(entryRowMessages(t, res)); got != 3 {
		t.Errorf("search by project name returned %d entries, want 3", got)
	}

	res = callTool(t, c, "list_entries", map[string]interface{}{
		"project_id": projectID,
		"sort_by":    "created_at",
		"order":      "asc",
	})
	if got, want := entryRowMessages(t, res), []string{"sprint planning", "invoice run", "code review"}; !equalStrings(got, want) {
		t.Errorf("ascending messages = %v, want %v", got, want)
	}

	res = callTool(t, c, "list_entries", map[string]interface{}{"sort_by": "cost"})
	if !res.IsError {
		t.Error("list_entries with an unknown sort_by: expected a tool error")
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
