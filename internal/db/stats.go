package db

import (
	"time"

	bolt "go.etcd.io/bbolt"
)

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

// GetStatistics calculates aggregated statistics with optional filtering. It
// shares the entry filter with the list queries, so both always agree on what
// a filter selects.
func (s *Store) GetStatistics(projectID string, startDate, endDate *time.Time, invoicedFilter *bool) (*Statistics, error) {
	q := EntryQuery{ProjectID: projectID, StartDate: startDate, EndDate: endDate, Invoiced: invoicedFilter}

	stats := &Statistics{
		ProjectBreakdown: make(map[string]int64),
	}

	err := s.view(func(tx *bolt.Tx) error {
		entries, err := collectEntries(tx, q, "", nil)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			stats.TotalMinutes += entry.Duration
			stats.EntryCount++

			if entry.Invoiced {
				stats.InvoicedMinutes += entry.Duration
			} else {
				stats.UninvoicedMinutes += entry.Duration
			}

			stats.ProjectBreakdown[entry.ProjectID] += entry.Duration

			if stats.EarliestEntry == nil || entry.CreatedAt.Before(*stats.EarliestEntry) {
				earliest := entry.CreatedAt
				stats.EarliestEntry = &earliest
			}
			if stats.LatestEntry == nil || entry.CreatedAt.After(*stats.LatestEntry) {
				latest := entry.CreatedAt
				stats.LatestEntry = &latest
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	stats.TotalHours = float64(stats.TotalMinutes) / 60.0
	return stats, nil
}
