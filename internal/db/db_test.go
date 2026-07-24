package db

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestDefaultPathHonoursEnv covers the resolver's precedence. It never touches
// the resolved path, only the string.
func TestDefaultPathHonoursEnv(t *testing.T) {
	t.Setenv(dbPathEnv, "/tmp/clockwork-test/custom.db")

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if want := "/tmp/clockwork-test/custom.db"; got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

// TestDefaultPathEmptyEnvFallsThrough pins that an empty value counts as unset.
func TestDefaultPathEmptyEnvFallsThrough(t *testing.T) {
	t.Setenv(dbPathEnv, "")

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join(".local", "clockwork", "default.db")) {
		t.Errorf("DefaultPath() = %q, want the default location", got)
	}
}

// TestNewCreatesMissingDirectory covers the bootstrap open: a read-only open
// cannot create the file, so New must.
func TestNewCreatesMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "clockwork.db")

	store, err := New(path)
	if err != nil {
		t.Fatalf("New(%q) error = %v", path, err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.ListProjects(); err != nil {
		t.Fatalf("ListProjects() on a fresh database error = %v", err)
	}
}

// TestSecondHandleSeesCommittedWrites is the point of the per-operation
// connection model: an idle store holds no lock, so a second handle on the same
// file (in production, the other process) can both read and write.
func TestSecondHandleSeesCommittedWrites(t *testing.T) {
	first, path := setupTestDB(t)
	t.Cleanup(func() { _ = first.Close() })

	second, err := New(path)
	if err != nil {
		t.Fatalf("second New(%q) error = %v", path, err)
	}
	t.Cleanup(func() { _ = second.Close() })

	project, err := first.CreateProject(ProjectInput{Name: "Shared"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	got, err := second.GetProject(project.ID)
	if err != nil {
		t.Fatalf("second handle GetProject() error = %v", err)
	}
	if got.Name != "Shared" {
		t.Errorf("Name = %q, want %q", got.Name, "Shared")
	}

	// And the write direction: the second handle's entry is visible to the first.
	if _, err := second.CreateEntry(project.ID, 30, "from the other handle", "", false, time.Now()); err != nil {
		t.Fatalf("second handle CreateEntry() error = %v", err)
	}
	entries, err := first.ListEntries(project.ID)
	if err != nil {
		t.Fatalf("ListEntries() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
}

// TestConcurrentWritersRetry checks that colliding writers back off and retry
// instead of failing, which is what makes running the TUI and the MCP server at
// the same time safe.
func TestConcurrentWritersRetry(t *testing.T) {
	store, path := setupTestDB(t)
	t.Cleanup(func() { _ = store.Close() })

	project, err := store.CreateProject(ProjectInput{Name: "Contended"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	const writers = 8
	var wg sync.WaitGroup
	errs := make(chan error, writers)

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// A separate handle per writer, so each takes the file lock on its own.
			writer, err := New(path)
			if err != nil {
				errs <- err
				return
			}
			defer writer.Close()

			if _, err := writer.CreateEntry(project.ID, int64(i+1), "concurrent", "", false, time.Now()); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent write failed: %v", err)
	}

	entries, err := store.ListEntries(project.ID)
	if err != nil {
		t.Fatalf("ListEntries() error = %v", err)
	}
	if len(entries) != writers {
		t.Errorf("len(entries) = %d, want %d", len(entries), writers)
	}
}

// TestTxIDAdvancesOnWrite covers the change-detection signal a long-lived
// reader polls.
func TestTxIDAdvancesOnWrite(t *testing.T) {
	store, _ := setupTestDB(t)
	t.Cleanup(func() { _ = store.Close() })

	before, err := store.TxID()
	if err != nil {
		t.Fatalf("TxID() error = %v", err)
	}

	if _, err := store.CreateProject(ProjectInput{Name: "Bump"}); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	after, err := store.TxID()
	if err != nil {
		t.Fatalf("TxID() error = %v", err)
	}
	if after <= before {
		t.Errorf("TxID() = %d after a write, want > %d", after, before)
	}
}
