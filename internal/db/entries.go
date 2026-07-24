package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/techthos/clockwork/internal/models"
	bolt "go.etcd.io/bbolt"
)

// Page sizing for entry queries. Over-max requests are clamped, never
// rejected. Exported so the MCP tool schema can document the real ceiling.
const (
	MaxEntryPageSize     = 50
	DefaultEntryPageSize = 50
)

// EntrySort names an ordering for QueryEntries. The empty value means the
// default, creation time.
type EntrySort string

const (
	EntrySortCreated  EntrySort = "created_at"
	EntrySortUpdated  EntrySort = "updated_at"
	EntrySortDuration EntrySort = "duration"
	EntrySortMessage  EntrySort = "message"
)

// ErrInvalidEntrySort is the sentinel behind every rejected sort value, so
// callers can match with errors.Is instead of on message text.
var ErrInvalidEntrySort = errors.New("invalid entry sort")

// EntrySorts lists the accepted sort values, for error messages and tool
// descriptions.
func EntrySorts() []EntrySort {
	return []EntrySort{EntrySortCreated, EntrySortUpdated, EntrySortDuration, EntrySortMessage}
}

// Valid reports whether s is a known sort. The empty value is valid and means
// the default.
func (s EntrySort) Valid() bool {
	if s == "" {
		return true
	}
	for _, known := range EntrySorts() {
		if s == known {
			return true
		}
	}
	return false
}

// EntryQuery is the filter/order/page criteria for QueryEntries. Zero values
// mean "no filter" and the criteria compose with AND.
type EntryQuery struct {
	ProjectID string     // empty = all projects
	StartDate *time.Time // inclusive lower bound on CreatedAt
	EndDate   *time.Time // inclusive upper bound on CreatedAt
	Invoiced  *bool      // nil = both

	// Search is a case-insensitive substring matched against the message, the
	// commit hash and the owning project's name.
	Search string

	SortBy EntrySort
	Asc    bool // false (the zero value) means descending, i.e. newest first

	Page     int // 1-based; < 1 means 1
	PageSize int // clamped to [1, maxEntryPageSize]; 0 means the default
}

// EntryPage is one page of entries plus metadata describing the full filtered
// set, so a caller can walk the rest.
type EntryPage struct {
	Entries    []*models.Entry `json:"entries"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	Total      int             `json:"total"`
	TotalPages int             `json:"total_pages"`
	HasMore    bool            `json:"has_more"`
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

	err := s.update(func(tx *bolt.Tx) error {
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

	err := s.view(func(tx *bolt.Tx) error {
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

	err := s.update(func(tx *bolt.Tx) error {
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
	return s.update(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}
		return b.Delete([]byte(id))
	})
}

// ListEntries returns all entries for a project, unpaginated. Internal callers
// (TUI, aggregation) use it when they want the whole set.
func (s *Store) ListEntries(projectID string) ([]*models.Entry, error) {
	return s.ListEntriesFiltered(projectID, nil, nil, nil)
}

// ListEntriesFiltered returns entries with optional filtering, unpaginated.
func (s *Store) ListEntriesFiltered(projectID string, startDate, endDate *time.Time, invoicedFilter *bool) ([]*models.Entry, error) {
	q := EntryQuery{ProjectID: projectID, StartDate: startDate, EndDate: endDate, Invoiced: invoicedFilter}

	var entries []*models.Entry
	err := s.view(func(tx *bolt.Tx) error {
		var err error
		entries, err = collectEntries(tx, q, "", nil)
		return err
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// QueryEntries returns one page of the entries matching q, ordered as q asks.
// The scan is in-memory: at single-user scale (hundreds to low thousands of
// rows) that is cheaper than maintaining an index, and there is none in v1.
func (s *Store) QueryEntries(q EntryQuery) (EntryPage, error) {
	if !q.SortBy.Valid() {
		return EntryPage{}, fmt.Errorf("%w: %q (want one of %s)", ErrInvalidEntrySort, q.SortBy, joinSorts(EntrySorts()))
	}
	if q.StartDate != nil && q.EndDate != nil && q.StartDate.After(*q.EndDate) {
		return EntryPage{}, fmt.Errorf("start date %s is after end date %s", q.StartDate.Format(time.RFC3339), q.EndDate.Format(time.RFC3339))
	}

	sortBy := q.SortBy
	if sortBy == "" {
		sortBy = EntrySortCreated
	}

	page := q.Page
	if page < 1 {
		page = 1
	}

	size := q.PageSize
	switch {
	case size < 1:
		size = DefaultEntryPageSize
	case size > MaxEntryPageSize:
		size = MaxEntryPageSize // clamp, don't error
	}

	search := strings.ToLower(strings.TrimSpace(q.Search))

	var matched []*models.Entry
	err := s.view(func(tx *bolt.Tx) error {
		// Only a search needs the project names, so only a search pays for the
		// extra bucket scan.
		var names map[string]string
		if search != "" {
			var err error
			if names, err = projectNames(tx); err != nil {
				return err
			}
		}

		var err error
		matched, err = collectEntries(tx, q, search, names)
		return err
	})
	if err != nil {
		return EntryPage{}, err
	}

	sortEntries(matched, sortBy, q.Asc)

	total := len(matched)
	totalPages := (total + size - 1) / size

	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}

	entries := make([]*models.Entry, 0, end-start)
	entries = append(entries, matched[start:end]...)

	return EntryPage{
		Entries:    entries,
		Page:       page,
		PageSize:   size,
		Total:      total,
		TotalPages: totalPages,
		HasMore:    end < total,
	}, nil
}

// collectEntries scans the entries bucket once and returns everything matching
// q. names may be nil when there is no search term to resolve.
func collectEntries(tx *bolt.Tx, q EntryQuery, search string, names map[string]string) ([]*models.Entry, error) {
	b, err := bucket(tx, entriesBucket)
	if err != nil {
		return nil, err
	}

	var matched []*models.Entry
	err = b.ForEach(func(k, v []byte) error {
		if v == nil {
			return nil // nested bucket, not an entry
		}
		var entry models.Entry
		if err := json.Unmarshal(v, &entry); err != nil {
			return fmt.Errorf("decode entry %q: %w", k, err)
		}
		if !matchesEntry(&entry, q, search, names) {
			return nil
		}
		// Unmarshalled into a local, so the value never outlives the txn.
		row := entry
		matched = append(matched, &row)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return matched, nil
}

// matchesEntry applies every criterion of q with AND.
func matchesEntry(entry *models.Entry, q EntryQuery, search string, names map[string]string) bool {
	if q.ProjectID != "" && entry.ProjectID != q.ProjectID {
		return false
	}
	if q.StartDate != nil && entry.CreatedAt.Before(*q.StartDate) {
		return false
	}
	if q.EndDate != nil && entry.CreatedAt.After(*q.EndDate) {
		return false
	}
	if q.Invoiced != nil && entry.Invoiced != *q.Invoiced {
		return false
	}
	if search == "" {
		return true
	}

	if strings.Contains(strings.ToLower(entry.Message), search) ||
		strings.Contains(strings.ToLower(entry.CommitHash), search) {
		return true
	}
	return strings.Contains(strings.ToLower(names[entry.ProjectID]), search)
}

// sortEntries orders entries by the requested key, tie-breaking on ID so the
// order is a strict total order and paging never repeats or drops a row.
func sortEntries(entries []*models.Entry, sortBy EntrySort, asc bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		c := compareEntries(entries[i], entries[j], sortBy)
		if c == 0 {
			c = strings.Compare(entries[i].ID, entries[j].ID)
		}
		if asc {
			return c < 0
		}
		return c > 0
	})
}

func compareEntries(a, b *models.Entry, sortBy EntrySort) int {
	switch sortBy {
	case EntrySortUpdated:
		return compareTimes(a.UpdatedAt, b.UpdatedAt)
	case EntrySortDuration:
		switch {
		case a.Duration < b.Duration:
			return -1
		case a.Duration > b.Duration:
			return 1
		}
		return 0
	case EntrySortMessage:
		return strings.Compare(strings.ToLower(a.Message), strings.ToLower(b.Message))
	default:
		return compareTimes(a.CreatedAt, b.CreatedAt)
	}
}

func compareTimes(a, b time.Time) int {
	switch {
	case a.Before(b):
		return -1
	case a.After(b):
		return 1
	}
	return 0
}

func joinSorts(sorts []EntrySort) string {
	parts := make([]string, 0, len(sorts))
	for _, s := range sorts {
		parts = append(parts, string(s))
	}
	return strings.Join(parts, ", ")
}

// GetLastEntry returns the most recent entry for a project in a single pass.
func (s *Store) GetLastEntry(projectID string) (*models.Entry, error) {
	var latest *models.Entry

	err := s.view(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}
		return b.ForEach(func(k, v []byte) error {
			if v == nil {
				return nil
			}
			var entry models.Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				return fmt.Errorf("decode entry %q: %w", k, err)
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

	err := s.view(func(tx *bolt.Tx) error {
		b, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}
		return b.ForEach(func(k, v []byte) error {
			if v == nil {
				return nil
			}
			var entry models.Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				return fmt.Errorf("decode entry %q: %w", k, err)
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
