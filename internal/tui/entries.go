package tui

import (
	"fmt"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/techthos/clockwork/internal/models"
)

// FilterOptions holds the current filter state for entries
type FilterOptions struct {
	ProjectID      string
	StartDate      *time.Time
	EndDate        *time.Time
	InvoicedFilter *bool // nil = all, true = invoiced only, false = uninvoiced only
}

func (a *App) createEntriesView(projectID string) tview.Primitive {
	// Initialize filter options
	filterOptions := &FilterOptions{
		ProjectID: projectID,
	}

	// Create table for entries list
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)

	// Create summary text view
	summaryView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	summaryView.SetBorderPadding(1, 0, 1, 1)

	// Create flex layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Get project name for header
	projectName := "All Projects"
	if projectID != "" {
		if project, err := a.store.GetProject(projectID); err == nil {
			projectName = project.Name
		}
	}

	// Header with title and instructions
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	header.SetText(fmt.Sprintf("[::b]Entries - %s[::-]\n", projectName) +
		"[gray]n: New | e: Edit | d: Delete | i: Toggle Invoiced | f: Filter | s: Stats | q: Back")
	header.SetBorderPadding(1, 1, 0, 0)

	flex.AddItem(header, 4, 0, false)
	flex.AddItem(table, 0, 1, true)
	flex.AddItem(summaryView, 3, 0, false)

	// Load and display entries
	loadEntries := func() {
		// Remember currently selected entry ID before clearing
		var selectedEntryID string
		row, _ := table.GetSelection()
		if row > 0 {
			cell := table.GetCell(row, 0)
			if entry, ok := cell.Reference.(*models.Entry); ok {
				selectedEntryID = entry.ID
			}
		}

		table.Clear()

		entries, err := a.store.ListEntriesFiltered(
			filterOptions.ProjectID,
			filterOptions.StartDate,
			filterOptions.EndDate,
			filterOptions.InvoicedFilter,
		)
		if err != nil {
			a.ShowErrorModal(fmt.Sprintf("Failed to load entries: %v", err), nil)
			return
		}

		// Sort entries by date (newest first)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].CreatedAt.After(entries[j].CreatedAt)
		})

		// Set table headers
		table.SetCell(0, 0, tview.NewTableCell("Date").
			SetTextColor(ColorTableHeader).
			SetAlign(tview.AlignLeft).
			SetSelectable(false))
		table.SetCell(0, 1, tview.NewTableCell("Duration").
			SetTextColor(ColorTableHeader).
			SetAlign(tview.AlignRight).
			SetSelectable(false))
		table.SetCell(0, 2, tview.NewTableCell("Message").
			SetTextColor(ColorTableHeader).
			SetAlign(tview.AlignLeft).
			SetSelectable(false))
		table.SetCell(0, 3, tview.NewTableCell("Invoiced").
			SetTextColor(ColorTableHeader).
			SetAlign(tview.AlignCenter).
			SetSelectable(false))

		// Calculate summary
		var totalMinutes int64
		var invoicedMinutes int64
		var uninvoicedMinutes int64

		// Track which row contains the previously selected entry
		rowToSelect := 1

		// Add entry rows
		for i, entry := range entries {
			row := i + 1

			// Update summary
			totalMinutes += entry.Duration
			if entry.Invoiced {
				invoicedMinutes += entry.Duration
			} else {
				uninvoicedMinutes += entry.Duration
			}

			// Check if this is the previously selected entry
			if selectedEntryID != "" && entry.ID == selectedEntryID {
				rowToSelect = row
			}

			// Format invoiced status
			invoicedText := "✗"
			invoicedColor := ColorUninvoiced
			if entry.Invoiced {
				invoicedText = "✓"
				invoicedColor = ColorInvoiced
			}

			table.SetCell(row, 0, tview.NewTableCell(FormatDate(entry.CreatedAt)).
				SetTextColor(ColorTableText).
				SetReference(entry))
			table.SetCell(row, 1, tview.NewTableCell(FormatDuration(entry.Duration)).
				SetTextColor(ColorTableText).
				SetAlign(tview.AlignRight))
			table.SetCell(row, 2, tview.NewTableCell(TruncateString(entry.Message, 60)).
				SetTextColor(ColorTableText))
			table.SetCell(row, 3, tview.NewTableCell(invoicedText).
				SetTextColor(invoicedColor).
				SetAlign(tview.AlignCenter))
		}

		// Update summary
		summaryText := fmt.Sprintf("[::b]Total: %s[::-] (%d entries) | ",
			FormatDuration(totalMinutes), len(entries))
		summaryText += fmt.Sprintf("[green]Invoiced: %s[::-] | [yellow]Uninvoiced: %s[::-]",
			FormatDuration(invoicedMinutes), FormatDuration(uninvoicedMinutes))

		summaryView.SetText(summaryText)

		// If no entries, show message
		if len(entries) == 0 {
			table.SetCell(1, 0, tview.NewTableCell("No entries found. Press 'n' to create one.").
				SetTextColor(ColorInfo).
				SetAlign(tview.AlignCenter))
		}

		// Select appropriate row (previously selected entry or first row)
		if len(entries) > 0 {
			table.Select(rowToSelect, 0)
		}
	}

	// Set up keyboard shortcuts
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			a.ShowProjectsView()
			return nil
		case 'n':
			a.ShowEntryForm(nil, projectID, loadEntries)
			return nil
		case 'e':
			row, _ := table.GetSelection()
			if row > 0 {
				cell := table.GetCell(row, 0)
				if entry, ok := cell.Reference.(*models.Entry); ok {
					a.ShowEntryForm(entry, "", loadEntries)
				}
			}
			return nil
		case 'd':
			row, _ := table.GetSelection()
			if row > 0 {
				cell := table.GetCell(row, 0)
				if entry, ok := cell.Reference.(*models.Entry); ok {
					a.confirmDeleteEntry(entry, loadEntries)
				}
			}
			return nil
		case 'i':
			row, _ := table.GetSelection()
			if row > 0 {
				cell := table.GetCell(row, 0)
				if entry, ok := cell.Reference.(*models.Entry); ok {
					a.toggleInvoiced(entry, loadEntries)
				}
			}
			return nil
		case 'f':
			a.ShowFilterModal(filterOptions, loadEntries)
			return nil
		case 's':
			a.ShowStatsView(projectID, filterOptions)
			return nil
		}

		switch event.Key() {
		case tcell.KeyCtrlC, tcell.KeyCtrlQ:
			a.Stop()
			return nil
		}

		return event
	})

	loadEntries()
	return flex
}

func (a *App) confirmDeleteEntry(entry *models.Entry, onComplete func()) {
	message := fmt.Sprintf("Delete entry from %s?", FormatDate(entry.CreatedAt))
	a.ShowConfirmModal(message,
		func() {
			if err := a.store.DeleteEntry(entry.ID); err != nil {
				a.ShowErrorModal(fmt.Sprintf("Failed to delete entry: %v", err), nil)
			} else {
				onComplete()
			}
		},
		nil,
	)
}

func (a *App) toggleInvoiced(entry *models.Entry, onComplete func()) {
	newInvoiced := !entry.Invoiced
	if _, err := a.store.UpdateEntry(entry.ID, nil, nil, nil, &newInvoiced, nil); err != nil {
		a.ShowErrorModal(fmt.Sprintf("Failed to update entry: %v", err), nil)
	} else {
		onComplete()
	}
}

// ShowFilterModal displays the filter configuration modal
func (a *App) ShowFilterModal(filterOptions *FilterOptions, onComplete func()) {
	form := tview.NewForm()

	// Get list of projects for dropdown
	projects, err := a.store.ListProjects()
	if err != nil {
		a.ShowErrorModal(fmt.Sprintf("Failed to load projects: %v", err), nil)
		return
	}

	// Build project options (include "All Projects")
	projectOptions := []string{"All Projects"}
	projectIDs := []string{""}
	selectedProjectIndex := 0

	for i, project := range projects {
		projectOptions = append(projectOptions, project.Name)
		projectIDs = append(projectIDs, project.ID)
		if project.ID == filterOptions.ProjectID {
			selectedProjectIndex = i + 1
		}
	}

	// Date range fields
	startDateStr := ""
	endDateStr := ""
	if filterOptions.StartDate != nil {
		startDateStr = FormatDate(*filterOptions.StartDate)
	}
	if filterOptions.EndDate != nil {
		endDateStr = FormatDate(*filterOptions.EndDate)
	}

	// Invoiced filter options
	invoicedOptions := []string{"All", "Invoiced Only", "Uninvoiced Only"}
	selectedInvoicedIndex := 0
	if filterOptions.InvoicedFilter != nil {
		if *filterOptions.InvoicedFilter {
			selectedInvoicedIndex = 1
		} else {
			selectedInvoicedIndex = 2
		}
	}

	// Project filter
	form.AddDropDown("Project", projectOptions, selectedProjectIndex, func(option string, optionIndex int) {
		filterOptions.ProjectID = projectIDs[optionIndex]
	})

	// Date range filters
	form.AddInputField("Start Date (YYYY-MM-DD)", startDateStr, 20, nil, func(text string) {
		startDateStr = text
	})

	form.AddInputField("End Date (YYYY-MM-DD)", endDateStr, 20, nil, func(text string) {
		endDateStr = text
	})

	// Invoiced filter
	form.AddDropDown("Invoiced Status", invoicedOptions, selectedInvoicedIndex, func(option string, optionIndex int) {
		switch optionIndex {
		case 0:
			filterOptions.InvoicedFilter = nil
		case 1:
			invoicedTrue := true
			filterOptions.InvoicedFilter = &invoicedTrue
		case 2:
			invoicedFalse := false
			filterOptions.InvoicedFilter = &invoicedFalse
		}
	})

	// Buttons
	form.AddButton("Apply", func() {
		// Parse dates
		if startDateStr != "" {
			startDate, err := time.Parse("2006-01-02", startDateStr)
			if err != nil {
				a.ShowErrorModal("Invalid start date format. Use YYYY-MM-DD", nil)
				return
			}
			filterOptions.StartDate = &startDate
		} else {
			filterOptions.StartDate = nil
		}

		if endDateStr != "" {
			endDate, err := time.Parse("2006-01-02", endDateStr)
			if err != nil {
				a.ShowErrorModal("Invalid end date format. Use YYYY-MM-DD", nil)
				return
			}
			// Set to end of day
			endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filterOptions.EndDate = &endDate
		} else {
			filterOptions.EndDate = nil
		}

		a.HideModal("filter_modal")
		if onComplete != nil {
			onComplete()
		}
	})

	form.AddButton("Clear Filters", func() {
		filterOptions.ProjectID = ""
		filterOptions.StartDate = nil
		filterOptions.EndDate = nil
		filterOptions.InvoicedFilter = nil
		a.HideModal("filter_modal")
		if onComplete != nil {
			onComplete()
		}
	})

	form.AddButton("Cancel", func() {
		a.HideModal("filter_modal")
	})

	form.SetBorder(true).
		SetTitle("Filter Entries").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(ColorPrimary)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.HideModal("filter_modal")
			return nil
		}
		return event
	})

	// Center the form
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 18, 1, true).
			AddItem(nil, 0, 1, false), 80, 1, true).
		AddItem(nil, 0, 1, false)

	a.ShowModal("filter_modal", modal)
}
