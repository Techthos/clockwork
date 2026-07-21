package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/techthos/clockwork/internal/models"
	"github.com/techthos/clockwork/internal/source"
	bolt "go.etcd.io/bbolt"
)

const (
	projectsBucket = "projects"
	entriesBucket  = "entries"
)

// Store manages database operations for clockwork
type Store struct {
	db *bolt.DB
}

// DefaultPath returns the canonical clockwork database path.
// Honours the CLOCKWORK_DB env var, otherwise falls back to
// ~/.local/clockwork/default.db.
func DefaultPath() (string, error) {
	if p := os.Getenv("CLOCKWORK_DB"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, ".local", "clockwork", "default.db"), nil
}

// New creates a new Store instance and initializes the database
func New(dbPath string) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// Open database
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(projectsBucket)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(entriesBucket)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// bucket safely retrieves a bucket and returns an error if missing.
func bucket(tx *bolt.Tx, name string) (*bolt.Bucket, error) {
	b := tx.Bucket([]byte(name))
	if b == nil {
		return nil, fmt.Errorf("bucket %q not found (database not initialized?)", name)
	}
	return b, nil
}

// validateCommitHash rejects empty/short hashes only when we have something
// long enough to validate; catches the e8e8 corruption pattern and obvious
// repetition. Hashes shorter than 40 chars (e.g. test fixtures) are allowed.
func validateCommitHash(hash string) error {
	if hash == "" || len(hash) < 40 {
		return nil
	}
	// Repetition check - first half identical to second half indicates corruption.
	if hash[:20] == hash[20:40] {
		return fmt.Errorf("invalid commit hash: repeated pattern detected - possible corruption (hash: %s)", hash)
	}
	// Specific e8e8 corruption pattern.
	if len(hash) == 40 && hash[20:] == "e8e8e8e8e8e8e8e8e8e8" {
		return fmt.Errorf("invalid commit hash: e8e8 corruption pattern detected (hash: %s)", hash)
	}
	return nil
}

// ProjectInput carries the fields needed to create a project. SourceType may
// be left empty, in which case it is inferred from whichever locator is set.
type ProjectInput struct {
	Name        string
	SourceType  string
	GitRepoPath string
	Repository  string
}

// ProjectUpdate carries partial project changes. A nil field is left
// untouched; a non-nil field is written, including when it is empty, which is
// how a locator gets cleared.
type ProjectUpdate struct {
	Name        *string
	SourceType  *string
	GitRepoPath *string
	Repository  *string
}

// normalizeProject fills in the effective source type for rows written before
// source_type existed, so callers always observe a concrete value.
func normalizeProject(p *models.Project) *models.Project {
	p.SourceType = source.Resolve(p).String()
	return p
}

// CreateProject creates a new project
func (s *Store) CreateProject(in ProjectInput) (*models.Project, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("project name must not be empty")
	}

	var srcType source.Type
	if raw := strings.TrimSpace(in.SourceType); raw != "" {
		parsed, err := source.Parse(raw)
		if err != nil {
			return nil, err
		}
		srcType = parsed
	} else {
		srcType = source.Infer(in.GitRepoPath, in.Repository)
	}

	if err := source.Validate(srcType, in.GitRepoPath, in.Repository); err != nil {
		return nil, err
	}

	project := &models.Project{
		ID:          uuid.New().String(),
		Name:        name,
		SourceType:  srcType.String(),
		GitRepoPath: in.GitRepoPath,
		Repository:  in.Repository,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		data, err := json.Marshal(project)
		if err != nil {
			return err
		}
		return b.Put([]byte(project.ID), data)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return project, nil
}

// GetProject retrieves a project by ID
func (s *Store) GetProject(id string) (*models.Project, error) {
	var project models.Project

	err := s.db.View(func(tx *bolt.Tx) error {
		b, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project not found")
		}
		return json.Unmarshal(data, &project)
	})

	if err != nil {
		return nil, err
	}

	return normalizeProject(&project), nil
}

// UpdateProject applies a partial update to an existing project. Switching
// source type clears the locator that no longer applies, so a project can move
// between lookup methods without leaving stale fields behind.
func (s *Store) UpdateProject(id string, upd ProjectUpdate) (*models.Project, error) {
	var project models.Project

	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project not found")
		}

		if err := json.Unmarshal(data, &project); err != nil {
			return err
		}
		normalizeProject(&project)

		if upd.Name != nil {
			if strings.TrimSpace(*upd.Name) == "" {
				return fmt.Errorf("project name must not be empty")
			}
			project.Name = strings.TrimSpace(*upd.Name)
		}

		srcType := source.Resolve(&project)
		if upd.SourceType != nil {
			parsed, err := source.Parse(*upd.SourceType)
			if err != nil {
				return err
			}
			srcType = parsed
		}

		// Reject locators that contradict the resolved type outright; silently
		// drop leftovers from the previous type further down.
		if upd.GitRepoPath != nil && *upd.GitRepoPath != "" && srcType != source.Local {
			return fmt.Errorf("git_repo_path only applies to source type %q, not %q", source.Local, srcType)
		}
		if upd.Repository != nil && *upd.Repository != "" && srcType != source.MCP {
			return fmt.Errorf("repository only applies to source type %q, not %q", source.MCP, srcType)
		}
		if upd.GitRepoPath != nil {
			project.GitRepoPath = *upd.GitRepoPath
		}
		if upd.Repository != nil {
			project.Repository = *upd.Repository
		}

		switch srcType {
		case source.None:
			project.GitRepoPath, project.Repository = "", ""
		case source.Local:
			project.Repository = ""
		case source.MCP:
			project.GitRepoPath = ""
		}

		if err := source.Validate(srcType, project.GitRepoPath, project.Repository); err != nil {
			return err
		}
		project.SourceType = srcType.String()
		project.UpdatedAt = time.Now()

		updatedData, err := json.Marshal(project)
		if err != nil {
			return err
		}

		return b.Put([]byte(id), updatedData)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	return &project, nil
}

// DeleteProject deletes a project and all its entries (single-pass cascade).
func (s *Store) DeleteProject(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		pb, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		if pb.Get([]byte(id)) == nil {
			return fmt.Errorf("project not found")
		}
		if err := pb.Delete([]byte(id)); err != nil {
			return err
		}

		eb, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}

		// Collect matching keys first; deleting during cursor iteration is unsafe.
		var toDelete [][]byte
		if err := eb.ForEach(func(k, v []byte) error {
			var entry models.Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				return nil // skip corrupt rows
			}
			if entry.ProjectID == id {
				kCopy := make([]byte, len(k))
				copy(kCopy, k)
				toDelete = append(toDelete, kCopy)
			}
			return nil
		}); err != nil {
			return err
		}

		for _, k := range toDelete {
			if err := eb.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListProjects returns all projects
func (s *Store) ListProjects() ([]*models.Project, error) {
	var projects []*models.Project

	err := s.db.View(func(tx *bolt.Tx) error {
		b, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		return b.ForEach(func(k, v []byte) error {
			var project models.Project
			if err := json.Unmarshal(v, &project); err != nil {
				return err
			}
			projects = append(projects, normalizeProject(&project))
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return projects, nil
}

// CreateEntry creates a new worklog entry
func (s *Store) CreateEntry(projectID string, duration int64, message, commitHash string, invoiced bool, createdAt time.Time) (*models.Entry, error) {
	if err := validateCommitHash(commitHash); err != nil {
		return nil, err
	}

	entry := &models.Entry{
		ID:         uuid.New().String(),
		ProjectID:  projectID,
		Duration:   duration,
		Message:    message,
		CommitHash: commitHash,
		Invoiced:   invoiced,
		CreatedAt:  createdAt,
		UpdatedAt:  time.Now(),
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		// Verify project exists inside the same transaction.
		pb, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		if pb.Get([]byte(projectID)) == nil {
			return fmt.Errorf("project not found: %s", projectID)
		}

		eb, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return eb.Put([]byte(entry.ID), data)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create entry: %w", err)
	}

	return entry, nil
}

// GetEntry retrieves an entry by ID
func (s *Store) GetEntry(id string) (*models.Entry, error) {
	var entry models.Entry

	err := s.db.View(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("entry not found")
		}
		return json.Unmarshal(data, &entry)
	})

	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// UpdateEntry updates an existing entry
func (s *Store) UpdateEntry(id string, duration *int64, message, commitHash *string, invoiced *bool, createdAt *time.Time) (*models.Entry, error) {
	if commitHash != nil {
		if err := validateCommitHash(*commitHash); err != nil {
			return nil, err
		}
	}

	var entry models.Entry

	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("entry not found")
		}

		if err := json.Unmarshal(data, &entry); err != nil {
			return err
		}

		if duration != nil {
			entry.Duration = *duration
		}
		if message != nil {
			entry.Message = *message
		}
		if commitHash != nil {
			entry.CommitHash = *commitHash
		}
		if invoiced != nil {
			entry.Invoiced = *invoiced
		}
		if createdAt != nil {
			entry.CreatedAt = *createdAt
		}
		entry.UpdatedAt = time.Now()

		updatedData, err := json.Marshal(entry)
		if err != nil {
			return err
		}

		return b.Put([]byte(id), updatedData)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update entry: %w", err)
	}

	return &entry, nil
}

// DeleteEntry deletes an entry
func (s *Store) DeleteEntry(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}
		return b.Delete([]byte(id))
	})
}

// ListEntries returns all entries for a project
func (s *Store) ListEntries(projectID string) ([]*models.Entry, error) {
	var entries []*models.Entry

	err := s.db.View(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}
		return b.ForEach(func(k, v []byte) error {
			var entry models.Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				return err
			}
			if entry.ProjectID == projectID {
				entries = append(entries, &entry)
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// GetLastEntry returns the most recent entry for a project in a single pass.
func (s *Store) GetLastEntry(projectID string) (*models.Entry, error) {
	var latest *models.Entry

	err := s.db.View(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}
		return b.ForEach(func(k, v []byte) error {
			var entry models.Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				return err
			}
			if entry.ProjectID != projectID {
				return nil
			}
			if latest == nil || entry.CreatedAt.After(latest.CreatedAt) {
				e := entry
				latest = &e
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return latest, nil
}

// GetLastCommitHash returns the most recent non-empty commit hash across all entries
// for a project in a single bucket pass.
func (s *Store) GetLastCommitHash(projectID string) (string, error) {
	var latestHash string
	var latestTime time.Time

	err := s.db.View(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}
		return b.ForEach(func(k, v []byte) error {
			var entry models.Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				return err
			}
			if entry.ProjectID != projectID || entry.CommitHash == "" {
				return nil
			}
			if latestHash == "" || entry.CreatedAt.After(latestTime) {
				latestHash = entry.CommitHash
				latestTime = entry.CreatedAt
			}
			return nil
		})
	})

	if err != nil {
		return "", err
	}

	return latestHash, nil
}

// ListEntriesFiltered returns entries with optional filtering
func (s *Store) ListEntriesFiltered(projectID string, startDate, endDate *time.Time, invoicedFilter *bool) ([]*models.Entry, error) {
	var entries []*models.Entry

	err := s.db.View(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}

		return b.ForEach(func(k, v []byte) error {
			var entry models.Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				return err
			}

			// Filter by project (empty = all projects)
			if projectID != "" && entry.ProjectID != projectID {
				return nil
			}

			// Filter by date range
			if startDate != nil && entry.CreatedAt.Before(*startDate) {
				return nil
			}
			if endDate != nil && entry.CreatedAt.After(*endDate) {
				return nil
			}

			// Filter by invoiced status (nil = all entries)
			if invoicedFilter != nil && entry.Invoiced != *invoicedFilter {
				return nil
			}

			entries = append(entries, &entry)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// Statistics represents aggregated entry statistics
type Statistics struct {
	TotalMinutes      int64            `json:"total_minutes"`
	TotalHours        float64          `json:"total_hours"`
	EntryCount        int              `json:"entry_count"`
	InvoicedMinutes   int64            `json:"invoiced_minutes"`
	UninvoicedMinutes int64            `json:"uninvoiced_minutes"`
	ProjectBreakdown  map[string]int64 `json:"project_breakdown"` // projectID -> minutes
	EarliestEntry     *time.Time       `json:"earliest_entry,omitempty"`
	LatestEntry       *time.Time       `json:"latest_entry,omitempty"`
}

// GetStatistics calculates aggregated statistics with optional filtering
func (s *Store) GetStatistics(projectID string, startDate, endDate *time.Time, invoicedFilter *bool) (*Statistics, error) {
	stats := &Statistics{
		ProjectBreakdown: make(map[string]int64),
	}

	err := s.db.View(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}

		return b.ForEach(func(k, v []byte) error {
			var entry models.Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				return err
			}

			// Filter by project (empty = all projects)
			if projectID != "" && entry.ProjectID != projectID {
				return nil
			}

			// Filter by date range
			if startDate != nil && entry.CreatedAt.Before(*startDate) {
				return nil
			}
			if endDate != nil && entry.CreatedAt.After(*endDate) {
				return nil
			}

			// Filter by invoiced status (nil = all entries)
			if invoicedFilter != nil && entry.Invoiced != *invoicedFilter {
				return nil
			}

			// Aggregate statistics
			stats.TotalMinutes += entry.Duration
			stats.EntryCount++

			if entry.Invoiced {
				stats.InvoicedMinutes += entry.Duration
			} else {
				stats.UninvoicedMinutes += entry.Duration
			}

			// Project breakdown
			stats.ProjectBreakdown[entry.ProjectID] += entry.Duration

			// Track earliest and latest entries
			if stats.EarliestEntry == nil || entry.CreatedAt.Before(*stats.EarliestEntry) {
				earliestTime := entry.CreatedAt
				stats.EarliestEntry = &earliestTime
			}
			if stats.LatestEntry == nil || entry.CreatedAt.After(*stats.LatestEntry) {
				latestTime := entry.CreatedAt
				stats.LatestEntry = &latestTime
			}

			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	stats.TotalHours = float64(stats.TotalMinutes) / 60.0
	return stats, nil
}
