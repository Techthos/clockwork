package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/techthos/clockwork/internal/models"
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

// CreateProject creates a new project
func (s *Store) CreateProject(name, gitRepoPath string) (*models.Project, error) {
	project := &models.Project{
		ID:          uuid.New().String(),
		Name:        name,
		GitRepoPath: gitRepoPath,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(projectsBucket))
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
		b := tx.Bucket([]byte(projectsBucket))
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project not found")
		}
		return json.Unmarshal(data, &project)
	})

	if err != nil {
		return nil, err
	}

	return &project, nil
}

// UpdateProject updates an existing project
func (s *Store) UpdateProject(id, name, gitRepoPath string) (*models.Project, error) {
	var project models.Project

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(projectsBucket))
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project not found")
		}

		if err := json.Unmarshal(data, &project); err != nil {
			return err
		}

		if name != "" {
			project.Name = name
		}
		if gitRepoPath != "" {
			project.GitRepoPath = gitRepoPath
		}
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

// DeleteProject deletes a project and all its entries
func (s *Store) DeleteProject(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Delete project
		pb := tx.Bucket([]byte(projectsBucket))
		if err := pb.Delete([]byte(id)); err != nil {
			return err
		}

		// Delete associated entries
		eb := tx.Bucket([]byte(entriesBucket))
		c := eb.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var entry models.Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue
			}
			if entry.ProjectID == id {
				if err := eb.Delete(k); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// ListProjects returns all projects
func (s *Store) ListProjects() ([]*models.Project, error) {
	var projects []*models.Project

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(projectsBucket))
		return b.ForEach(func(k, v []byte) error {
			var project models.Project
			if err := json.Unmarshal(v, &project); err != nil {
				return err
			}
			projects = append(projects, &project)
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
	// Verify project exists
	if _, err := s.GetProject(projectID); err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	// Validate commit hash for corruption patterns
	if commitHash != "" && len(commitHash) >= 40 {
		// Check for suspicious patterns in full-length hashes
		// This specifically catches the e8e8e8e8 corruption bug
		firstHalf := commitHash[:20]
		secondHalf := commitHash[20:40]
		if firstHalf == secondHalf {
			return nil, fmt.Errorf("invalid commit hash: repeated pattern detected - possible corruption (hash: %s)", commitHash)
		}
		// Check for the specific e8 repetition pattern
		if len(commitHash) == 40 && commitHash[20:] == "e8e8e8e8e8e8e8e8e8e8" {
			return nil, fmt.Errorf("invalid commit hash: e8e8 corruption pattern detected (hash: %s)", commitHash)
		}
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
		b := tx.Bucket([]byte(entriesBucket))
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		return b.Put([]byte(entry.ID), data)
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
		b := tx.Bucket([]byte(entriesBucket))
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
	var entry models.Entry

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(entriesBucket))
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
			// Validate commit hash for corruption patterns
			newHash := *commitHash
			if newHash != "" && len(newHash) >= 40 {
				// Check for suspicious patterns in full-length hashes
				firstHalf := newHash[:20]
				secondHalf := newHash[20:40]
				if firstHalf == secondHalf {
					return fmt.Errorf("invalid commit hash: repeated pattern detected - possible corruption (hash: %s)", newHash)
				}
				// Check for the specific e8 repetition pattern
				if len(newHash) == 40 && newHash[20:] == "e8e8e8e8e8e8e8e8e8e8" {
					return fmt.Errorf("invalid commit hash: e8e8 corruption pattern detected (hash: %s)", newHash)
				}
			}
			entry.CommitHash = newHash
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
		b := tx.Bucket([]byte(entriesBucket))
		return b.Delete([]byte(id))
	})
}

// ListEntries returns all entries for a project
func (s *Store) ListEntries(projectID string) ([]*models.Entry, error) {
	var entries []*models.Entry

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(entriesBucket))
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

// GetLastEntry returns the most recent entry for a project
func (s *Store) GetLastEntry(projectID string) (*models.Entry, error) {
	entries, err := s.ListEntries(projectID)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Find most recent entry
	var latest *models.Entry
	for _, entry := range entries {
		if latest == nil || entry.CreatedAt.After(latest.CreatedAt) {
			latest = entry
		}
	}

	return latest, nil
}

// ListEntriesFiltered returns entries with optional filtering
func (s *Store) ListEntriesFiltered(projectID string, startDate, endDate *time.Time, invoicedFilter *bool) ([]*models.Entry, error) {
	var entries []*models.Entry

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(entriesBucket))
		if b == nil {
			return nil
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
		b := tx.Bucket([]byte(entriesBucket))
		if b == nil {
			return nil
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
