package tui

import (
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/techthos/clockwork/internal/models"
)

func (a *App) createProjectsView() tview.Primitive {
	// Create table for projects list
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)

	// Create flex layout with header and table
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Header with title and instructions
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	header.SetText("[::b]Clockwork - Project Management[::-]\n" +
		"[gray]n: New | e: Edit | d: Delete | Enter: View Entries | q: Quit")
	header.SetBorderPadding(1, 1, 0, 0)

	flex.AddItem(header, 4, 0, false)
	flex.AddItem(table, 0, 1, true)

	// Load and display projects
	loadProjects := func() {
		table.Clear()

		projects, err := a.store.ListProjects()
		if err != nil {
			a.ShowErrorModal(fmt.Sprintf("Failed to load projects: %v", err), nil)
			return
		}

		// Sort projects by name
		sort.Slice(projects, func(i, j int) bool {
			return projects[i].Name < projects[j].Name
		})

		// Set table headers
		table.SetCell(0, 0, tview.NewTableCell("Name").
			SetTextColor(ColorTableHeader).
			SetAlign(tview.AlignLeft).
			SetSelectable(false))
		table.SetCell(0, 1, tview.NewTableCell("Git Repository").
			SetTextColor(ColorTableHeader).
			SetAlign(tview.AlignLeft).
			SetSelectable(false))
		table.SetCell(0, 2, tview.NewTableCell("Created").
			SetTextColor(ColorTableHeader).
			SetAlign(tview.AlignLeft).
			SetSelectable(false))

		// Add project rows
		for i, project := range projects {
			row := i + 1
			table.SetCell(row, 0, tview.NewTableCell(project.Name).
				SetTextColor(ColorTableText).
				SetReference(project))
			table.SetCell(row, 1, tview.NewTableCell(project.GitRepoPath).
				SetTextColor(ColorTableText))
			table.SetCell(row, 2, tview.NewTableCell(FormatDate(project.CreatedAt)).
				SetTextColor(ColorTableText))
		}

		// If no projects, show message
		if len(projects) == 0 {
			table.SetCell(1, 0, tview.NewTableCell("No projects yet. Press 'n' to create one.").
				SetTextColor(ColorInfo).
				SetAlign(tview.AlignCenter))
		}

		// Select first data row
		if len(projects) > 0 {
			table.Select(1, 0)
		}
	}

	// Set up keyboard shortcuts
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			a.Stop()
			return nil
		case 'n':
			a.ShowProjectForm(nil, loadProjects)
			return nil
		case 'e':
			row, _ := table.GetSelection()
			if row > 0 {
				cell := table.GetCell(row, 0)
				if project, ok := cell.Reference.(*models.Project); ok {
					a.ShowProjectForm(project, loadProjects)
				}
			}
			return nil
		case 'd':
			row, _ := table.GetSelection()
			if row > 0 {
				cell := table.GetCell(row, 0)
				if project, ok := cell.Reference.(*models.Project); ok {
					a.confirmDeleteProject(project, loadProjects)
				}
			}
			return nil
		}

		switch event.Key() {
		case tcell.KeyEnter:
			row, _ := table.GetSelection()
			if row > 0 {
				cell := table.GetCell(row, 0)
				if project, ok := cell.Reference.(*models.Project); ok {
					a.ShowEntriesView(project.ID)
				}
			}
			return nil
		case tcell.KeyCtrlC, tcell.KeyCtrlQ:
			a.Stop()
			return nil
		}

		return event
	})

	loadProjects()
	return flex
}

func (a *App) confirmDeleteProject(project *models.Project, onComplete func()) {
	message := fmt.Sprintf("Delete project '%s' and all its entries?", project.Name)
	a.ShowConfirmModal(message,
		func() {
			// Confirmed - delete project
			if err := a.store.DeleteProject(project.ID); err != nil {
				a.ShowErrorModal(fmt.Sprintf("Failed to delete project: %v", err), nil)
			} else {
				onComplete()
			}
		},
		nil, // Cancel - do nothing
	)
}
