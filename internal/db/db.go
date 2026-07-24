// Package db is the only place that talks to bbolt. Callers get domain models
// back, never a transaction, bucket, or transaction-scoped byte slice.
package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
	bolterrors "go.etcd.io/bbolt/errors"
)

// Bucket names. Package-level []byte constants, never inline literals.
var (
	projectsBucket = []byte("projects")
	entriesBucket  = []byte("entries")
)

// dbPathEnv overrides the default database location.
const dbPathEnv = "CLOCKWORK_DB"

// Connection budget. bbolt takes its file lock at Open, not per transaction,
// so a short per-attempt timeout plus backoff turns a brief collision with the
// other process into a sub-second wait instead of a hard failure.
const (
	openTimeout    = 75 * time.Millisecond
	openBudget     = 3 * time.Second
	initialBackoff = 25 * time.Millisecond
	maxBackoff     = 250 * time.Millisecond
)

// Store manages database operations for clockwork.
//
// It holds the path, not an open handle: every operation opens bbolt, runs one
// short transaction and closes again — read-only for reads, read-write for
// writes. An idle process therefore holds no lock, which is what lets the TUI
// and the MCP server run as concurrent processes against one file. See
// docs/bbolt-concurrent-access-strategy.md.
type Store struct {
	path string
}

// DefaultPath reports where the database lives, honouring CLOCKWORK_DB. An
// empty value counts as unset. Resolution is pure: it computes a string and
// touches nothing on disk.
func DefaultPath() (string, error) {
	if p := os.Getenv(dbPathEnv); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for default db path: %w", err)
	}
	return filepath.Join(home, ".local", "clockwork", "default.db"), nil
}

// New creates a Store for dbPath and bootstraps the file: the one read-write
// open that creates it and its buckets, since a read-only open cannot create a
// missing file.
func New(dbPath string) (*Store, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("database path must not be empty")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory for %q: %w", dbPath, err)
	}

	store := &Store{path: dbPath}
	if err := store.bootstrap(); err != nil {
		return nil, err
	}
	return store, nil
}

// Path reports the file this store operates on.
func (s *Store) Path() string { return s.path }

// Close exists for symmetry with the callers' lifecycle handling. The store
// holds no handle between operations, so there is nothing to release.
func (s *Store) Close() error { return nil }

// bootstrap creates the required top-level buckets in a single migration
// transaction. Idempotent, so it runs on every startup.
func (s *Store) bootstrap() error {
	return s.update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{projectsBucket, entriesBucket} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return fmt.Errorf("create bucket %q: %w", name, err)
			}
		}
		return nil
	})
}

// open acquires a handle, retrying with backoff while the other process holds
// the lock. Only a lock-acquire timeout is retried; anything else fails now.
func (s *Store) open(readOnly bool) (*bolt.DB, error) {
	deadline := time.Now().Add(openBudget)
	backoff := initialBackoff

	for attempt := 1; ; attempt++ {
		bdb, err := bolt.Open(s.path, 0o600, &bolt.Options{Timeout: openTimeout, ReadOnly: readOnly})
		if err == nil {
			return bdb, nil
		}
		if !errors.Is(err, bolterrors.ErrTimeout) || !time.Now().Before(deadline) {
			return nil, fmt.Errorf("open bbolt at %q (readOnly=%v, attempts=%d): %w", s.path, readOnly, attempt, err)
		}
		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= 2
		}
	}
}

// view runs fn in a read transaction on a short-lived read-only handle. A
// read-only open takes a shared lock, so concurrent readers do not block each
// other.
func (s *Store) view(fn func(*bolt.Tx) error) error {
	bdb, err := s.open(true)
	if err != nil {
		return err
	}
	defer bdb.Close()

	return bdb.View(fn)
}

// update runs fn in a write transaction on a short-lived read-write handle,
// which holds the exclusive lock for the open-to-close span only. Keep fn
// short: no network, no user I/O.
func (s *Store) update(fn func(*bolt.Tx) error) error {
	bdb, err := s.open(false)
	if err != nil {
		return err
	}
	defer bdb.Close()

	return bdb.Update(fn)
}

// TxID reports bbolt's monotonic committed transaction id. It changes exactly
// when someone commits a write, so a long-lived reader (the TUI) can poll it
// to notice another process's changes without scanning any data.
func (s *Store) TxID() (int, error) {
	var id int
	err := s.view(func(tx *bolt.Tx) error {
		id = tx.ID()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

// bucket safely retrieves a bucket and returns an error if missing.
func bucket(tx *bolt.Tx, name []byte) (*bolt.Bucket, error) {
	b := tx.Bucket(name)
	if b == nil {
		return nil, fmt.Errorf("bucket %q not found (database not initialized?)", name)
	}
	return b, nil
}
