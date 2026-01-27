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

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
