package tui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/techthos/clockwork/internal/git"
	"github.com/techthos/clockwork/internal/models"
	"github.com/techthos/clockwork/internal/utils"
)

// ShowEntryForm displays the create/edit entry form
// If entry is nil, creates new entry; otherwise edits existing entry
// If defaultProjectID is provided (and entry is nil), pre-selects that project
func (a *App) ShowEntryForm(entry *models.Entry, defaultProjectID string, onComplete func()) {
	isEdit := entry != nil

	// If creating new entry, show mode selection modal first
	if !isEdit {
		a.showEntryModeSelection(defaultProjectID, onComplete)
		return
	}

	// Edit mode - show manual form with existing data
	a.showManualEntryForm(entry, onComplete)
}

func (a *App) showEntryModeSelection(defaultProjectID string, onComplete func()) {
	modal := tview.NewModal().
		SetText("Select entry creation mode:").
		AddButtons([]string{"Git (from commits)", "Manual", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.HideModal("entry_mode")
			switch buttonIndex {
			case 0:
				a.showGitEntryForm(defaultProjectID, onComplete)
			case 1:
				a.showManualEntryForm(nil, onComplete)
			}
		})

	modal.SetBackgroundColor(tcell.ColorDefault)
	modal.SetBorderColor(ColorPrimary)

	a.ShowModal("entry_mode", modal)
}

func (a *App) showGitEntryForm(defaultProjectID string, onComplete func()) {
	form := tview.NewForm()

	// Get list of projects
	projects, err := a.store.ListProjects()
	if err != nil {
		a.ShowErrorModal(fmt.Sprintf("Failed to load projects: %v", err), nil)
		return
	}

	if len(projects) == 0 {
		a.ShowErrorModal("No projects available. Create a project first.", nil)
		return
	}

	// Build project options
	projectOptions := make([]string, len(projects))
	projectMap := make(map[string]*models.Project)
	selectedIndex := 0

	for i, project := range projects {
		projectOptions[i] = project.Name
		projectMap[project.Name] = project
		if project.ID == defaultProjectID {
			selectedIndex = i
		}
	}

	var selectedProject *models.Project = projects[selectedIndex]
	var customDuration string
	var customMessage string
	invoiced := false

	// Project dropdown
	form.AddDropDown("Project", projectOptions, selectedIndex, func(option string, optionIndex int) {
		selectedProject = projectMap[option]
	})

	// Optional custom duration
	form.AddInputField("Custom Duration (optional)", "", 20,
		func(textToCheck string, lastChar rune) bool {
			return true // Allow any input, will validate on save
		},
		func(text string) {
			customDuration = text
		})

	// Optional custom message
	form.AddTextArea("Custom Message (optional)", "", 60, 3, 0, func(text string) {
		customMessage = text
	})

	// Invoiced checkbox
	form.AddCheckbox("Invoiced", false, func(checked bool) {
		invoiced = checked
	})

	// Add buttons
	form.AddButton("Create from Git", func() {
		if selectedProject == nil {
			a.ShowErrorModal("No project selected", nil)
			return
		}

		// Get last entry for this project
		lastEntry, err := a.store.GetLastEntry(selectedProject.ID)
		if err != nil {
			a.ShowErrorModal(fmt.Sprintf("Failed to get last entry: %v", err), nil)
			return
		}

		lastHash := ""
		if lastEntry != nil {
			lastHash = lastEntry.CommitHash
		}

		// Fetch commits since last entry
		commits, err := git.GetCommitsSince(selectedProject.GitRepoPath, lastHash)
		if err != nil {
			a.ShowErrorModal(fmt.Sprintf("Failed to fetch commits: %v", err), nil)
			return
		}

		if len(commits) == 0 {
			a.ShowErrorModal("No new commits since last entry", nil)
			return
		}

		// Calculate duration
		duration := git.CalculateDuration(commits)
		if customDuration != "" {
			parsedDuration, err := utils.ParseDuration(customDuration)
			if err != nil {
				a.ShowErrorModal(fmt.Sprintf("Invalid duration: %v", err), nil)
				return
			}
			duration = parsedDuration
		}

		// Generate message
		message := git.AggregateCommits(commits)
		if customMessage != "" {
			message = customMessage
		}

		// Get latest commit hash
		latestHash := commits[0].Hash

		// Create entry
		_, err = a.store.CreateEntry(
			selectedProject.ID,
			duration,
			message,
			latestHash,
			invoiced,
			time.Now(),
		)

		if err != nil {
			a.ShowErrorModal(fmt.Sprintf("Failed to create entry: %v", err), nil)
			return
		}

		a.HideModal("git_entry_form")
		if onComplete != nil {
			onComplete()
		}
	})

	form.AddButton("Cancel", func() {
		a.HideModal("git_entry_form")
	})

	form.SetBorder(true).
		SetTitle("New Entry (Git Mode)").
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(ColorPrimary)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.HideModal("git_entry_form")
			return nil
		}
		return event
	})

	// Center the form
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 20, 1, true).
			AddItem(nil, 0, 1, false), 80, 1, true).
		AddItem(nil, 0, 1, false)

	a.ShowModal("git_entry_form", modal)
}

func (a *App) showManualEntryForm(entry *models.Entry, onComplete func()) {
	form := tview.NewForm()

	isEdit := entry != nil
	title := "New Entry (Manual Mode)"
	if isEdit {
		title = "Edit Entry"
	}

	// Get list of projects
	projects, err := a.store.ListProjects()
	if err != nil {
		a.ShowErrorModal(fmt.Sprintf("Failed to load projects: %v", err), nil)
		return
	}

	if len(projects) == 0 {
		a.ShowErrorModal("No projects available. Create a project first.", nil)
		return
	}

	// Build project options
	projectOptions := make([]string, len(projects))
	projectMap := make(map[string]*models.Project)
	selectedIndex := 0

	for i, project := range projects {
		projectOptions[i] = project.Name
		projectMap[project.Name] = project
		if isEdit && project.ID == entry.ProjectID {
			selectedIndex = i
		}
	}

	var selectedProject *models.Project = projects[selectedIndex]
	durationField := ""
	messageField := ""
	commitHashField := ""
	invoiced := false

	if isEdit {
		durationField = FormatDuration(entry.Duration)
		messageField = entry.Message
		commitHashField = entry.CommitHash
		invoiced = entry.Invoiced
	}

	// Project dropdown
	form.AddDropDown("Project", projectOptions, selectedIndex, func(option string, optionIndex int) {
		selectedProject = projectMap[option]
	})

	// Duration field
	form.AddInputField("Duration (e.g., 1h 30m, 90m)", durationField, 20,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			durationField = text
		})

	// Message field
	form.AddTextArea("Message", messageField, 60, 5, 0, func(text string) {
		messageField = text
	})

	// Commit Hash field (optional)
	form.AddInputField("Commit Hash (optional)", commitHashField, 50,
		func(textToCheck string, lastChar rune) bool {
			return true
		},
		func(text string) {
			commitHashField = text
		})

	// Invoiced checkbox
	form.AddCheckbox("Invoiced", invoiced, func(checked bool) {
		invoiced = checked
	})

	// Add buttons
	saveButtonLabel := "Create"
	if isEdit {
		saveButtonLabel = "Save"
	}

	form.AddButton(saveButtonLabel, func() {
		// Validate inputs
		if durationField == "" {
			a.ShowErrorModal("Duration cannot be empty", nil)
			return
		}
		if messageField == "" {
			a.ShowErrorModal("Message cannot be empty", nil)
			return
		}

		// Parse duration
		duration, err := utils.ParseDuration(durationField)
		if err != nil {
			a.ShowErrorModal(fmt.Sprintf("Invalid duration: %v", err), nil)
			return
		}

		if isEdit {
			// Update existing entry
			_, err = a.store.UpdateEntry(entry.ID, &duration, &messageField, &commitHashField, &invoiced, nil)
			if err != nil {
				a.ShowErrorModal(fmt.Sprintf("Failed to update entry: %v", err), nil)
				return
			}
		} else {
			// Create new entry
			_, err = a.store.CreateEntry(
				selectedProject.ID,
				duration,
				messageField,
				commitHashField,
				invoiced,
				time.Now(),
			)
			if err != nil {
				a.ShowErrorModal(fmt.Sprintf("Failed to create entry: %v", err), nil)
				return
			}
		}

		a.HideModal("manual_entry_form")
		if onComplete != nil {
			onComplete()
		}
	})

	form.AddButton("Cancel", func() {
		a.HideModal("manual_entry_form")
	})

	form.SetBorder(true).
		SetTitle(title).
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(ColorPrimary)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.HideModal("manual_entry_form")
			return nil
		}
		return event
	})

	// Center the form
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 20, 1, true).
			AddItem(nil, 0, 1, false), 80, 1, true).
		AddItem(nil, 0, 1, false)

	a.ShowModal("manual_entry_form", modal)
}
