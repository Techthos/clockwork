package db

import (
	"errors"
	"testing"
	"time"
)

// seedEntries builds one project with entries spaced a day apart, newest last.
func seedEntries(t *testing.T, store *Store, projectName string, messages []string) string {
	t.Helper()

	project, err := store.CreateProject(ProjectInput{Name: projectName})
	if err != nil {
		t.Fatalf("CreateProject(%q) error = %v", projectName, err)
	}

	base := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	for i, msg := range messages {
		if _, err := store.CreateEntry(project.ID, int64(30*(i+1)), msg, "", i%2 == 0, base.AddDate(0, 0, i)); err != nil {
			t.Fatalf("CreateEntry(%q) error = %v", msg, err)
		}
	}
	return project.ID
}

func TestQueryEntriesDefaultsToNewestFirst(t *testing.T) {
	store, _ := setupTestDB(t)
	t.Cleanup(func() { _ = store.Close() })

	projectID := seedEntries(t, store, "Alpha", []string{"first", "second", "third"})

	page, err := store.QueryEntries(EntryQuery{ProjectID: projectID})
	if err != nil {
		t.Fatalf("QueryEntries() error = %v", err)
	}

	if page.Total != 3 || page.TotalPages != 1 || page.HasMore {
		t.Errorf("page meta = {total:%d, total_pages:%d, has_more:%v}, want {3, 1, false}",
			page.Total, page.TotalPages, page.HasMore)
	}
	if page.PageSize != DefaultEntryPageSize || page.Page != 1 {
		t.Errorf("page/size = %d/%d, want 1/%d", page.Page, page.PageSize, DefaultEntryPageSize)
	}
	if got := page.Entries[0].Message; got != "third" {
		t.Errorf("first entry = %q, want %q (newest first is the default)", got, "third")
	}
}

func TestQueryEntriesPagination(t *testing.T) {
	store, _ := setupTestDB(t)
	t.Cleanup(func() { _ = store.Close() })

	projectID := seedEntries(t, store, "Alpha", []string{"a", "b", "c", "d", "e"})

	tests := []struct {
		name        string
		page        int
		pageSize    int
		wantLen     int
		wantSize    int
		wantPages   int
		wantHasMore bool
		wantFirst   string
	}{
		{name: "first page", page: 1, pageSize: 2, wantLen: 2, wantSize: 2, wantPages: 3, wantHasMore: true, wantFirst: "e"},
		{name: "middle page", page: 2, pageSize: 2, wantLen: 2, wantSize: 2, wantPages: 3, wantHasMore: true, wantFirst: "c"},
		{name: "last partial page", page: 3, pageSize: 2, wantLen: 1, wantSize: 2, wantPages: 3, wantFirst: "a"},
		{name: "past the end", page: 9, pageSize: 2, wantLen: 0, wantSize: 2, wantPages: 3},
		{name: "page below one is clamped up", page: 0, pageSize: 5, wantLen: 5, wantSize: 5, wantPages: 1, wantFirst: "e"},
		{name: "size above max is clamped down", page: 1, pageSize: 5000, wantLen: 5, wantSize: MaxEntryPageSize, wantPages: 1, wantFirst: "e"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := store.QueryEntries(EntryQuery{ProjectID: projectID, Page: tc.page, PageSize: tc.pageSize})
			if err != nil {
				t.Fatalf("QueryEntries() error = %v", err)
			}
			if len(page.Entries) != tc.wantLen {
				t.Errorf("len(entries) = %d, want %d", len(page.Entries), tc.wantLen)
			}
			if page.PageSize != tc.wantSize {
				t.Errorf("page_size = %d, want %d", page.PageSize, tc.wantSize)
			}
			if page.TotalPages != tc.wantPages {
				t.Errorf("total_pages = %d, want %d", page.TotalPages, tc.wantPages)
			}
			if page.HasMore != tc.wantHasMore {
				t.Errorf("has_more = %v, want %v", page.HasMore, tc.wantHasMore)
			}
			if tc.wantFirst != "" && page.Entries[0].Message != tc.wantFirst {
				t.Errorf("first entry = %q, want %q", page.Entries[0].Message, tc.wantFirst)
			}
		})
	}
}

func TestQueryEntriesSearch(t *testing.T) {
	store, _ := setupTestDB(t)
	t.Cleanup(func() { _ = store.Close() })

	alpha := seedEntries(t, store, "Alpha Client", []string{"invoice run", "sprint planning"})
	seedEntries(t, store, "Beta Client", []string{"refactor db layer"})

	if _, err := store.CreateEntry(alpha, 15, "hash carrier", "9f1c2d3e4b5a60718293a4b5c6d7e8f901a2b3c4", false, time.Now()); err != nil {
		t.Fatalf("CreateEntry() error = %v", err)
	}

	tests := []struct {
		name    string
		search  string
		wantLen int
	}{
		{name: "matches message case-insensitively", search: "SPRINT", wantLen: 1},
		{name: "matches the owning project name", search: "beta", wantLen: 1},
		{name: "matches a commit hash prefix", search: "9f1c2d3e", wantLen: 1},
		{name: "blank search means no search", search: "   ", wantLen: 4},
		{name: "no match", search: "nothing here", wantLen: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := store.QueryEntries(EntryQuery{Search: tc.search})
			if err != nil {
				t.Fatalf("QueryEntries() error = %v", err)
			}
			if len(page.Entries) != tc.wantLen {
				t.Errorf("len(entries) = %d, want %d", len(page.Entries), tc.wantLen)
			}
		})
	}
}

func TestQueryEntriesSorting(t *testing.T) {
	store, _ := setupTestDB(t)
	t.Cleanup(func() { _ = store.Close() })

	// Durations rise with the index, so duration order and creation order agree.
	projectID := seedEntries(t, store, "Alpha", []string{"charlie", "alpha", "bravo"})

	tests := []struct {
		name      string
		sortBy    EntrySort
		asc       bool
		wantFirst string
	}{
		{name: "created descending is the default direction", sortBy: EntrySortCreated, wantFirst: "bravo"},
		{name: "created ascending", sortBy: EntrySortCreated, asc: true, wantFirst: "charlie"},
		{name: "longest first", sortBy: EntrySortDuration, wantFirst: "bravo"},
		{name: "shortest first", sortBy: EntrySortDuration, asc: true, wantFirst: "charlie"},
		{name: "message ascending", sortBy: EntrySortMessage, asc: true, wantFirst: "alpha"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := store.QueryEntries(EntryQuery{ProjectID: projectID, SortBy: tc.sortBy, Asc: tc.asc})
			if err != nil {
				t.Fatalf("QueryEntries() error = %v", err)
			}
			if page.Entries[0].Message != tc.wantFirst {
				t.Errorf("first entry = %q, want %q", page.Entries[0].Message, tc.wantFirst)
			}
		})
	}
}

func TestQueryEntriesRejectsUnknownSort(t *testing.T) {
	store, _ := setupTestDB(t)
	t.Cleanup(func() { _ = store.Close() })

	seedEntries(t, store, "Alpha", []string{"one"})

	_, err := store.QueryEntries(EntryQuery{SortBy: EntrySort("cost")})
	if !errors.Is(err, ErrInvalidEntrySort) {
		t.Errorf("QueryEntries() error = %v, want ErrInvalidEntrySort", err)
	}
}

func TestQueryEntriesFiltersCompose(t *testing.T) {
	store, _ := setupTestDB(t)
	t.Cleanup(func() { _ = store.Close() })

	// Entries land on 2026-01-01, -02, -03; invoiced on the even indexes.
	projectID := seedEntries(t, store, "Alpha", []string{"one", "two", "three"})
	seedEntries(t, store, "Beta", []string{"other"})

	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	invoiced := true

	page, err := store.QueryEntries(EntryQuery{
		ProjectID: projectID,
		StartDate: &start,
		Invoiced:  &invoiced,
	})
	if err != nil {
		t.Fatalf("QueryEntries() error = %v", err)
	}
	if len(page.Entries) != 1 || page.Entries[0].Message != "three" {
		t.Fatalf("entries = %+v, want only the invoiced entry after the start date", page.Entries)
	}
}

func TestQueryEntriesRejectsInvertedDateRange(t *testing.T) {
	store, _ := setupTestDB(t)
	t.Cleanup(func() { _ = store.Close() })

	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, -7)

	if _, err := store.QueryEntries(EntryQuery{StartDate: &start, EndDate: &end}); err == nil {
		t.Error("QueryEntries() with start after end: expected an error")
	}
}
