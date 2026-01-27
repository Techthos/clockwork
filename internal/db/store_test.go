package db

import (
	"os"
	"path/filepath"
	"testing"
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

	entry, err := store.CreateEntry(project.ID, 120, "Test work", "abc123", false)
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
	entry, _ := store.CreateEntry(project.ID, 60, "Original", "abc", false)

	newDuration := int64(120)
	newMessage := "Updated"
	invoiced := true

	updated, err := store.UpdateEntry(entry.ID, &newDuration, &newMessage, nil, &invoiced)
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
	store.CreateEntry(project.ID, 60, "Entry 1", "abc", false)
	store.CreateEntry(project.ID, 90, "Entry 2", "def", false)

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
	store.CreateEntry(project.ID, 60, "Entry 1", "abc", false)
	entry2, _ := store.CreateEntry(project.ID, 90, "Entry 2", "def", false)

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
	store.CreateEntry(project.ID, 60, "Entry 1", "abc", false)
	store.CreateEntry(project.ID, 90, "Entry 2", "def", false)

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
	store.CreateEntry(project1.ID, 60, "Entry 1 - Not Invoiced", "abc", false)
	store.CreateEntry(project1.ID, 90, "Entry 2 - Invoiced", "def", true)
	store.CreateEntry(project2.ID, 120, "Entry 3 - Not Invoiced", "ghi", false)
	store.CreateEntry(project2.ID, 150, "Entry 4 - Invoiced", "jkl", true)

	// Test: List all entries (no filters)
	allEntries, err := store.ListEntriesFiltered("", nil)
	if err != nil {
		t.Fatalf("Failed to list all entries: %v", err)
	}
	if len(allEntries) != 4 {
		t.Errorf("Expected 4 entries total, got %d", len(allEntries))
	}

	// Test: List all entries for project 1
	project1Entries, err := store.ListEntriesFiltered(project1.ID, nil)
	if err != nil {
		t.Fatalf("Failed to list project 1 entries: %v", err)
	}
	if len(project1Entries) != 2 {
		t.Errorf("Expected 2 entries for project 1, got %d", len(project1Entries))
	}

	// Test: List all invoiced entries
	invoicedTrue := true
	invoicedEntries, err := store.ListEntriesFiltered("", &invoicedTrue)
	if err != nil {
		t.Fatalf("Failed to list invoiced entries: %v", err)
	}
	if len(invoicedEntries) != 2 {
		t.Errorf("Expected 2 invoiced entries, got %d", len(invoicedEntries))
	}

	// Test: List all not invoiced entries
	invoicedFalse := false
	notInvoicedEntries, err := store.ListEntriesFiltered("", &invoicedFalse)
	if err != nil {
		t.Fatalf("Failed to list not invoiced entries: %v", err)
	}
	if len(notInvoicedEntries) != 2 {
		t.Errorf("Expected 2 not invoiced entries, got %d", len(notInvoicedEntries))
	}

	// Test: List not invoiced entries for project 1
	project1NotInvoiced, err := store.ListEntriesFiltered(project1.ID, &invoicedFalse)
	if err != nil {
		t.Fatalf("Failed to list project 1 not invoiced entries: %v", err)
	}
	if len(project1NotInvoiced) != 1 {
		t.Errorf("Expected 1 not invoiced entry for project 1, got %d", len(project1NotInvoiced))
	}

	// Test: List invoiced entries for project 2
	project2Invoiced, err := store.ListEntriesFiltered(project2.ID, &invoicedTrue)
	if err != nil {
		t.Fatalf("Failed to list project 2 invoiced entries: %v", err)
	}
	if len(project2Invoiced) != 1 {
		t.Errorf("Expected 1 invoiced entry for project 2, got %d", len(project2Invoiced))
	}
}

func TestGetStatistics(t *testing.T) {
	store, _ := setupTestDB(t)
	defer store.Close()

	project1, _ := store.CreateProject("Project 1", "/path/1")
	project2, _ := store.CreateProject("Project 2", "/path/2")

	// Create entries with different properties
	store.CreateEntry(project1.ID, 60, "Entry 1", "abc", false)   // 1 hour, not invoiced
	store.CreateEntry(project1.ID, 90, "Entry 2", "def", true)    // 1.5 hours, invoiced
	store.CreateEntry(project2.ID, 120, "Entry 3", "ghi", false)  // 2 hours, not invoiced
	store.CreateEntry(project2.ID, 150, "Entry 4", "jkl", true)   // 2.5 hours, invoiced

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

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
