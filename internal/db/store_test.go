package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*Store, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	return store, dbPath
}

func TestCreateAndGetProject(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project, err := store.CreateProject("Test Project", "/path/to/repo")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	if project.Name != "Test Project" {
		t.Errorf("Expected name 'Test Project', got '%s'", project.Name)
	}

	retrieved, err := store.GetProject(project.ID)
	if err != nil {
		t.Fatalf("Failed to get project: %v", err)
	}

	if retrieved.ID != project.ID {
		t.Errorf("Expected ID '%s', got '%s'", project.ID, retrieved.ID)
	}
}

func TestUpdateProject(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project, _ := store.CreateProject("Original", "/path/1")

	updated, err := store.UpdateProject(project.ID, "Updated", "/path/2")
	if err != nil {
		t.Fatalf("Failed to update project: %v", err)
	}

	if updated.Name != "Updated" {
		t.Errorf("Expected name 'Updated', got '%s'", updated.Name)
	}

	if updated.GitRepoPath != "/path/2" {
		t.Errorf("Expected path '/path/2', got '%s'", updated.GitRepoPath)
	}
}

func TestDeleteProject(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project, _ := store.CreateProject("To Delete", "/path")

	err := store.DeleteProject(project.ID)
	if err != nil {
		t.Fatalf("Failed to delete project: %v", err)
	}

	_, err = store.GetProject(project.ID)
	if err == nil {
		t.Error("Expected error when getting deleted project, got nil")
	}
}

func TestListProjects(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	store.CreateProject("Project 1", "/path/1")
	store.CreateProject("Project 2", "/path/2")

	projects, err := store.ListProjects()
	if err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(projects))
	}
}

func TestCreateAndGetEntry(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project, _ := store.CreateProject("Test", "/path")

	entry, err := store.CreateEntry(project.ID, 120, "Test work", "abc123", false, time.Now())
	if err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}

	if entry.Duration != 120 {
		t.Errorf("Expected duration 120, got %d", entry.Duration)
	}

	retrieved, err := store.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("Failed to get entry: %v", err)
	}

	if retrieved.ID != entry.ID {
		t.Errorf("Expected ID '%s', got '%s'", entry.ID, retrieved.ID)
	}
}

func TestUpdateEntry(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project, _ := store.CreateProject("Test", "/path")
	entry, _ := store.CreateEntry(project.ID, 60, "Original", "abc", false, time.Now())

	newDuration := int64(120)
	newMessage := "Updated"
	invoiced := true

	updated, err := store.UpdateEntry(entry.ID, &newDuration, &newMessage, nil, &invoiced, nil)
	if err != nil {
		t.Fatalf("Failed to update entry: %v", err)
	}

	if updated.Duration != 120 {
		t.Errorf("Expected duration 120, got %d", updated.Duration)
	}

	if updated.Message != "Updated" {
		t.Errorf("Expected message 'Updated', got '%s'", updated.Message)
	}

	if !updated.Invoiced {
		t.Error("Expected invoiced to be true")
	}
}

func TestListEntries(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project, _ := store.CreateProject("Test", "/path")
	store.CreateEntry(project.ID, 60, "Entry 1", "abc", false, time.Now())
	store.CreateEntry(project.ID, 90, "Entry 2", "def", false, time.Now())

	entries, err := store.ListEntries(project.ID)
	if err != nil {
		t.Fatalf("Failed to list entries: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}
}

func TestGetLastEntry(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project, _ := store.CreateProject("Test", "/path")

	// No entries yet
	last, err := store.GetLastEntry(project.ID)
	if err != nil {
		t.Fatalf("Failed to get last entry: %v", err)
	}
	if last != nil {
		t.Error("Expected nil for project with no entries")
	}

	// Add entries
	store.CreateEntry(project.ID, 60, "Entry 1", "abc", false, time.Now())
	entry2, _ := store.CreateEntry(project.ID, 90, "Entry 2", "def", false, time.Now())

	last, err = store.GetLastEntry(project.ID)
	if err != nil {
		t.Fatalf("Failed to get last entry: %v", err)
	}

	if last.ID != entry2.ID {
		t.Errorf("Expected last entry ID '%s', got '%s'", entry2.ID, last.ID)
	}
}

func TestDatabasePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "persist.db")

	// Create and populate database
	store1, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	project, _ := store1.CreateProject("Persistent", "/path")
	store1.Close()

	// Reopen database
	store2, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer store2.Close()

	retrieved, err := store2.GetProject(project.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve project after reopen: %v", err)
	}

	if retrieved.Name != "Persistent" {
		t.Errorf("Expected name 'Persistent', got '%s'", retrieved.Name)
	}
}

func TestDeleteProjectCascade(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project, _ := store.CreateProject("Test", "/path")
	store.CreateEntry(project.ID, 60, "Entry 1", "abc", false, time.Now())
	store.CreateEntry(project.ID, 90, "Entry 2", "def", false, time.Now())

	err := store.DeleteProject(project.ID)
	if err != nil {
		t.Fatalf("Failed to delete project: %v", err)
	}

	entries, err := store.ListEntries(project.ID)
	if err != nil {
		t.Fatalf("Failed to list entries: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after project deletion, got %d", len(entries))
	}
}

func TestListEntriesFiltered(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project1, _ := store.CreateProject("Project 1", "/path/1")
	project2, _ := store.CreateProject("Project 2", "/path/2")

	// Create entries with different invoiced statuses
	store.CreateEntry(project1.ID, 60, "Entry 1 - Not Invoiced", "abc", false, time.Now())
	store.CreateEntry(project1.ID, 90, "Entry 2 - Invoiced", "def", true, time.Now())
	store.CreateEntry(project2.ID, 120, "Entry 3 - Not Invoiced", "ghi", false, time.Now())
	store.CreateEntry(project2.ID, 150, "Entry 4 - Invoiced", "jkl", true, time.Now())

	// Test: List all entries (no filters)
	allEntries, err := store.ListEntriesFiltered("", nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to list all entries: %v", err)
	}
	if len(allEntries) != 4 {
		t.Errorf("Expected 4 entries total, got %d", len(allEntries))
	}

	// Test: List all entries for project 1
	project1Entries, err := store.ListEntriesFiltered(project1.ID, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to list project 1 entries: %v", err)
	}
	if len(project1Entries) != 2 {
		t.Errorf("Expected 2 entries for project 1, got %d", len(project1Entries))
	}

	// Test: List all invoiced entries
	invoicedTrue := true
	invoicedEntries, err := store.ListEntriesFiltered("", nil, nil, &invoicedTrue)
	if err != nil {
		t.Fatalf("Failed to list invoiced entries: %v", err)
	}
	if len(invoicedEntries) != 2 {
		t.Errorf("Expected 2 invoiced entries, got %d", len(invoicedEntries))
	}

	// Test: List all not invoiced entries
	invoicedFalse := false
	notInvoicedEntries, err := store.ListEntriesFiltered("", nil, nil, &invoicedFalse)
	if err != nil {
		t.Fatalf("Failed to list not invoiced entries: %v", err)
	}
	if len(notInvoicedEntries) != 2 {
		t.Errorf("Expected 2 not invoiced entries, got %d", len(notInvoicedEntries))
	}

	// Test: List not invoiced entries for project 1
	project1NotInvoiced, err := store.ListEntriesFiltered(project1.ID, nil, nil, &invoicedFalse)
	if err != nil {
		t.Fatalf("Failed to list project 1 not invoiced entries: %v", err)
	}
	if len(project1NotInvoiced) != 1 {
		t.Errorf("Expected 1 not invoiced entry for project 1, got %d", len(project1NotInvoiced))
	}

	// Test: List invoiced entries for project 2
	project2Invoiced, err := store.ListEntriesFiltered(project2.ID, nil, nil, &invoicedTrue)
	if err != nil {
		t.Fatalf("Failed to list project 2 invoiced entries: %v", err)
	}
	if len(project2Invoiced) != 1 {
		t.Errorf("Expected 1 invoiced entry for project 2, got %d", len(project2Invoiced))
	}
}

func TestListEntriesFilteredByDateRange(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project1, _ := store.CreateProject("Project 1", "/path/1")
	project2, _ := store.CreateProject("Project 2", "/path/2")

	// Create entries with specific dates
	jan1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	jan15 := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	jan31 := time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC)
	feb15 := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	mar1 := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	store.CreateEntry(project1.ID, 60, "Jan 1 Entry", "abc", false, jan1)
	store.CreateEntry(project1.ID, 90, "Jan 15 Entry", "def", true, jan15)
	store.CreateEntry(project2.ID, 120, "Jan 31 Entry", "ghi", false, jan31)
	store.CreateEntry(project2.ID, 150, "Feb 15 Entry", "jkl", true, feb15)
	store.CreateEntry(project1.ID, 180, "Mar 1 Entry", "mno", false, mar1)

	// Test: List all entries in January
	janStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	janEnd := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)
	janEntries, err := store.ListEntriesFiltered("", &janStart, &janEnd, nil)
	if err != nil {
		t.Fatalf("Failed to list January entries: %v", err)
	}
	if len(janEntries) != 3 {
		t.Errorf("Expected 3 entries in January, got %d", len(janEntries))
	}

	// Test: List entries from Jan 15 onwards
	jan15Start := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	laterEntries, err := store.ListEntriesFiltered("", &jan15Start, nil, nil)
	if err != nil {
		t.Fatalf("Failed to list entries from Jan 15: %v", err)
	}
	if len(laterEntries) != 4 {
		t.Errorf("Expected 4 entries from Jan 15 onwards, got %d", len(laterEntries))
	}

	// Test: List entries until end of January
	earlierEntries, err := store.ListEntriesFiltered("", nil, &janEnd, nil)
	if err != nil {
		t.Fatalf("Failed to list entries until Jan 31: %v", err)
	}
	if len(earlierEntries) != 3 {
		t.Errorf("Expected 3 entries until Jan 31, got %d", len(earlierEntries))
	}

	// Test: List entries for specific date range (mid-Jan to mid-Feb)
	midJan := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	midFeb := time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC)
	midEntries, err := store.ListEntriesFiltered("", &midJan, &midFeb, nil)
	if err != nil {
		t.Fatalf("Failed to list mid-range entries: %v", err)
	}
	if len(midEntries) != 3 {
		t.Errorf("Expected 3 entries in mid-Jan to mid-Feb range, got %d", len(midEntries))
	}

	// Test: Combine date range with project filter
	project1JanEntries, err := store.ListEntriesFiltered(project1.ID, &janStart, &janEnd, nil)
	if err != nil {
		t.Fatalf("Failed to list project 1 January entries: %v", err)
	}
	if len(project1JanEntries) != 2 {
		t.Errorf("Expected 2 entries for project 1 in January, got %d", len(project1JanEntries))
	}

	// Test: Combine date range with invoiced filter
	invoicedTrue := true
	invoicedJanEntries, err := store.ListEntriesFiltered("", &janStart, &janEnd, &invoicedTrue)
	if err != nil {
		t.Fatalf("Failed to list invoiced January entries: %v", err)
	}
	if len(invoicedJanEntries) != 1 {
		t.Errorf("Expected 1 invoiced entry in January, got %d", len(invoicedJanEntries))
	}

	// Test: All filters combined (project + date range + invoiced status)
	invoicedFalse := false
	combinedEntries, err := store.ListEntriesFiltered(project1.ID, &janStart, &janEnd, &invoicedFalse)
	if err != nil {
		t.Fatalf("Failed to list combined filtered entries: %v", err)
	}
	if len(combinedEntries) != 1 {
		t.Errorf("Expected 1 entry for project 1, not invoiced, in January, got %d", len(combinedEntries))
	}

	// Test: Date range with no matching entries
	futureStart := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	futureEnd := time.Date(2027, 12, 31, 0, 0, 0, 0, time.UTC)
	futureEntries, err := store.ListEntriesFiltered("", &futureStart, &futureEnd, nil)
	if err != nil {
		t.Fatalf("Failed to list future entries: %v", err)
	}
	if len(futureEntries) != 0 {
		t.Errorf("Expected 0 entries in future date range, got %d", len(futureEntries))
	}
}

func TestGetStatistics(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project1, _ := store.CreateProject("Project 1", "/path/1")
	project2, _ := store.CreateProject("Project 2", "/path/2")

	// Create entries with different properties
	store.CreateEntry(project1.ID, 60, "Entry 1", "abc", false, time.Now())   // 1 hour, not invoiced
	store.CreateEntry(project1.ID, 90, "Entry 2", "def", true, time.Now())    // 1.5 hours, invoiced
	store.CreateEntry(project2.ID, 120, "Entry 3", "ghi", false, time.Now())  // 2 hours, not invoiced
	store.CreateEntry(project2.ID, 150, "Entry 4", "jkl", true, time.Now())   // 2.5 hours, invoiced

	// Test: All statistics (no filters)
	stats, err := store.GetStatistics("", nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to get statistics: %v", err)
	}

	expectedTotal := int64(60 + 90 + 120 + 150)
	if stats.TotalMinutes != expectedTotal {
		t.Errorf("Expected total minutes %d, got %d", expectedTotal, stats.TotalMinutes)
	}

	expectedHours := 7.0 // 420 minutes / 60
	if stats.TotalHours != expectedHours {
		t.Errorf("Expected total hours %.1f, got %.1f", expectedHours, stats.TotalHours)
	}

	if stats.EntryCount != 4 {
		t.Errorf("Expected 4 entries, got %d", stats.EntryCount)
	}

	expectedInvoiced := int64(90 + 150)
	if stats.InvoicedMinutes != expectedInvoiced {
		t.Errorf("Expected invoiced minutes %d, got %d", expectedInvoiced, stats.InvoicedMinutes)
	}

	expectedUninvoiced := int64(60 + 120)
	if stats.UninvoicedMinutes != expectedUninvoiced {
		t.Errorf("Expected uninvoiced minutes %d, got %d", expectedUninvoiced, stats.UninvoicedMinutes)
	}

	// Test: Project filter
	projectStats, err := store.GetStatistics(project1.ID, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to get project statistics: %v", err)
	}

	if projectStats.TotalMinutes != 150 {
		t.Errorf("Expected project 1 total 150 minutes, got %d", projectStats.TotalMinutes)
	}

	if projectStats.EntryCount != 2 {
		t.Errorf("Expected 2 entries for project 1, got %d", projectStats.EntryCount)
	}

	// Test: Invoiced filter
	invoicedTrue := true
	invoicedStats, err := store.GetStatistics("", nil, nil, &invoicedTrue)
	if err != nil {
		t.Fatalf("Failed to get invoiced statistics: %v", err)
	}

	if invoicedStats.TotalMinutes != 240 {
		t.Errorf("Expected invoiced total 240 minutes, got %d", invoicedStats.TotalMinutes)
	}

	if invoicedStats.EntryCount != 2 {
		t.Errorf("Expected 2 invoiced entries, got %d", invoicedStats.EntryCount)
	}

	// Test: Not invoiced filter
	invoicedFalse := false
	uninvoicedStats, err := store.GetStatistics("", nil, nil, &invoicedFalse)
	if err != nil {
		t.Fatalf("Failed to get uninvoiced statistics: %v", err)
	}

	if uninvoicedStats.TotalMinutes != 180 {
		t.Errorf("Expected uninvoiced total 180 minutes, got %d", uninvoicedStats.TotalMinutes)
	}

	// Test: Project breakdown
	if len(stats.ProjectBreakdown) != 2 {
		t.Errorf("Expected 2 projects in breakdown, got %d", len(stats.ProjectBreakdown))
	}

	if stats.ProjectBreakdown[project1.ID] != 150 {
		t.Errorf("Expected project 1 breakdown 150 minutes, got %d", stats.ProjectBreakdown[project1.ID])
	}

	if stats.ProjectBreakdown[project2.ID] != 270 {
		t.Errorf("Expected project 2 breakdown 270 minutes, got %d", stats.ProjectBreakdown[project2.ID])
	}

	// Test: Earliest and latest entries
	if stats.EarliestEntry == nil {
		t.Error("Expected earliest entry to be set")
	}

	if stats.LatestEntry == nil {
		t.Error("Expected latest entry to be set")
	}
}

func TestCreateEntryWithCustomDate(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project, _ := store.CreateProject("Test", "/path")

	// Create entry with custom date
	customDate := time.Date(2025, 12, 15, 14, 30, 0, 0, time.UTC)
	entry, err := store.CreateEntry(project.ID, 120, "Custom date entry", "abc123", false, customDate)
	if err != nil {
		t.Fatalf("Failed to create entry with custom date: %v", err)
	}

	// Verify the custom date was set correctly
	if !entry.CreatedAt.Equal(customDate) {
		t.Errorf("Expected CreatedAt %v, got %v", customDate, entry.CreatedAt)
	}

	// Retrieve and verify
	retrieved, err := store.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve entry: %v", err)
	}

	if !retrieved.CreatedAt.Equal(customDate) {
		t.Errorf("Expected retrieved CreatedAt %v, got %v", customDate, retrieved.CreatedAt)
	}
}

func TestUpdateEntryWithCustomDate(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project, _ := store.CreateProject("Test", "/path")

	// Create entry with current time
	originalDate := time.Now()
	entry, _ := store.CreateEntry(project.ID, 60, "Original entry", "abc", false, originalDate)

	// Update with custom date
	newDate := time.Date(2025, 11, 20, 10, 15, 0, 0, time.UTC)
	updated, err := store.UpdateEntry(entry.ID, nil, nil, nil, nil, &newDate)
	if err != nil {
		t.Fatalf("Failed to update entry with custom date: %v", err)
	}

	// Verify the date was updated
	if !updated.CreatedAt.Equal(newDate) {
		t.Errorf("Expected CreatedAt %v, got %v", newDate, updated.CreatedAt)
	}

	// Update without changing the date (nil parameter)
	newMessage := "Updated message"
	updated2, err := store.UpdateEntry(entry.ID, nil, &newMessage, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to update entry: %v", err)
	}

	// Verify date remained unchanged
	if !updated2.CreatedAt.Equal(newDate) {
		t.Errorf("Expected CreatedAt to remain %v, got %v", newDate, updated2.CreatedAt)
	}

	// Verify message was updated
	if updated2.Message != "Updated message" {
		t.Errorf("Expected message 'Updated message', got '%s'", updated2.Message)
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
