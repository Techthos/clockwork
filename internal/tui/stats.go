package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) createStatsView(projectID string, filterOptions *FilterOptions) tview.Primitive {
	// Create text view for statistics
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)

	// Create flex layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Header with title and instructions
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	header.SetText("[::b]Statistics[::-]\n" +
		"[gray]f: Filter | r: Refresh | q: Back")
	header.SetBorderPadding(1, 1, 0, 0)

	flex.AddItem(header, 4, 0, false)
	flex.AddItem(textView, 0, 1, true)

	// Load and display statistics
	loadStats := func() {
		textView.Clear()

		// Use filterOptions if provided, otherwise use current state
		var projID string

		if filterOptions != nil {
			projID = filterOptions.ProjectID
		} else {
			projID = projectID
		}

		// Determine which filter values to use
		var startDate, endDate *time.Time
		var invoicedFilter *bool

		if filterOptions != nil {
			startDate = filterOptions.StartDate
			endDate = filterOptions.EndDate
			invoicedFilter = filterOptions.InvoicedFilter
		}

		stats, err := a.store.GetStatistics(
			projID,
			startDate,
			endDate,
			invoicedFilter,
		)
		if err != nil {
			a.ShowErrorModal(fmt.Sprintf("Failed to load statistics: %v", err), nil)
			return
		}

		var builder strings.Builder

		// Overall statistics
		builder.WriteString("[::b]Overall Statistics[::-]\n\n")
		builder.WriteString(fmt.Sprintf("Total Time:          %s (%.2f hours)\n",
			FormatDuration(stats.TotalMinutes), stats.TotalHours))
		builder.WriteString(fmt.Sprintf("Entry Count:         %d\n\n", stats.EntryCount))

		// Date range
		if stats.EarliestEntry != nil && stats.LatestEntry != nil {
			builder.WriteString(fmt.Sprintf("Date Range:          %s to %s\n\n",
				FormatDate(*stats.EarliestEntry),
				FormatDate(*stats.LatestEntry)))
		}

		// Invoiced vs Uninvoiced breakdown
		builder.WriteString("[::b]Invoiced Status Breakdown[::-]\n\n")
		if stats.TotalMinutes > 0 {
			invoicedPct := FormatPercentage(float64(stats.InvoicedMinutes), float64(stats.TotalMinutes))
			uninvoicedPct := FormatPercentage(float64(stats.UninvoicedMinutes), float64(stats.TotalMinutes))

			builder.WriteString(fmt.Sprintf("[green]Invoiced:[::-]        %s (%.2f hours) - %s\n",
				FormatDuration(stats.InvoicedMinutes),
				float64(stats.InvoicedMinutes)/60.0,
				invoicedPct))
			builder.WriteString(fmt.Sprintf("[yellow]Uninvoiced:[::-]      %s (%.2f hours) - %s\n\n",
				FormatDuration(stats.UninvoicedMinutes),
				float64(stats.UninvoicedMinutes)/60.0,
				uninvoicedPct))
		} else {
			builder.WriteString("No data available\n\n")
		}

		// Project breakdown
		if len(stats.ProjectBreakdown) > 0 {
			builder.WriteString("[::b]Project Breakdown[::-]\n\n")

			// Sort projects by time (descending)
			type projectStat struct {
				id      string
				name    string
				minutes int64
			}
			var projectStats []projectStat

			for projectID, minutes := range stats.ProjectBreakdown {
				project, err := a.store.GetProject(projectID)
				name := "Unknown Project"
				if err == nil {
					name = project.Name
				}
				projectStats = append(projectStats, projectStat{
					id:      projectID,
					name:    name,
					minutes: minutes,
				})
			}

			sort.Slice(projectStats, func(i, j int) bool {
				return projectStats[i].minutes > projectStats[j].minutes
			})

			for _, ps := range projectStats {
				pct := FormatPercentage(float64(ps.minutes), float64(stats.TotalMinutes))
				builder.WriteString(fmt.Sprintf("%-30s %s (%.2f hours) - %s\n",
					TruncateString(ps.name, 30),
					FormatDuration(ps.minutes),
					float64(ps.minutes)/60.0,
					pct))
			}
		}

		// Active filters
		if filterOptions != nil {
			builder.WriteString("\n[::b]Active Filters[::-]\n\n")
			if filterOptions.ProjectID != "" {
				project, err := a.store.GetProject(filterOptions.ProjectID)
				if err == nil {
					builder.WriteString(fmt.Sprintf("Project: %s\n", project.Name))
				}
			} else {
				builder.WriteString("Project: All Projects\n")
			}

			if filterOptions.StartDate != nil {
				builder.WriteString(fmt.Sprintf("Start Date: %s\n", FormatDate(*filterOptions.StartDate)))
			}
			if filterOptions.EndDate != nil {
				builder.WriteString(fmt.Sprintf("End Date: %s\n", FormatDate(*filterOptions.EndDate)))
			}

			if filterOptions.InvoicedFilter != nil {
				if *filterOptions.InvoicedFilter {
					builder.WriteString("Status: Invoiced Only\n")
				} else {
					builder.WriteString("Status: Uninvoiced Only\n")
				}
			} else {
				builder.WriteString("Status: All\n")
			}
		}

		textView.SetText(builder.String())
	}

	// Set up keyboard shortcuts
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			// Go back to entries view
			if filterOptions != nil {
				a.ShowEntriesView(filterOptions.ProjectID)
			} else {
				a.ShowEntriesView(projectID)
			}
			return nil
		case 'r':
			loadStats()
			return nil
		case 'f':
			if filterOptions == nil {
				filterOptions = &FilterOptions{ProjectID: projectID}
			}
			a.ShowFilterModal(filterOptions, loadStats)
			return nil
		}

		switch event.Key() {
		case tcell.KeyCtrlC, tcell.KeyCtrlQ:
			a.Stop()
			return nil
		}

		return event
	})

	loadStats()
	return flex
}
