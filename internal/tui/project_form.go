package tui

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/techthos/clockwork/internal/models"
)

// ShowProjectForm displays the create/edit project form
func (a *App) ShowProjectForm(project *models.Project, onComplete func()) {
	form := tview.NewForm()

	// Determine if creating or editing
	isEdit := project != nil
	title := "New Project"
	if isEdit {
		title = "Edit Project"
	}

	// Set up form fields
	nameField := ""
	repoField := ""
	if isEdit {
		nameField = project.Name
		repoField = project.GitRepoPath
	}

	form.AddInputField("Name", nameField, 40, nil, func(text string) {
		nameField = text
	})

	form.AddInputField("Git Repository Path", repoField, 60, nil, func(text string) {
		repoField = text
	})

	// Add buttons
	form.AddButton("Save", func() {
		// Validate inputs
		if nameField == "" {
			a.ShowErrorModal("Project name cannot be empty", nil)
			return
		}
		if repoField == "" {
			a.ShowErrorModal("Git repository path cannot be empty", nil)
			return
		}

		// Validate git repo path
		if err := validateGitRepo(repoField); err != nil {
			a.ShowErrorModal(fmt.Sprintf("Invalid git repository: %v", err), nil)
			return
		}

		var err error
		if isEdit {
			_, err = a.store.UpdateProject(project.ID, nameField, repoField)
		} else {
			_, err = a.store.CreateProject(nameField, repoField)
		}

		if err != nil {
			a.ShowErrorModal(fmt.Sprintf("Failed to save project: %v", err), nil)
			return
		}

		a.HideModal("project_form")
		if onComplete != nil {
			onComplete()
		}
	})

	form.AddButton("Cancel", func() {
		a.HideModal("project_form")
	})

	form.SetBorder(true).
		SetTitle(title).
		SetTitleAlign(tview.AlignLeft).
		SetBorderColor(ColorPrimary)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.HideModal("project_form")
			return nil
		}
		return event
	})

	// Center the form
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 12, 1, true).
			AddItem(nil, 0, 1, false), 80, 1, true).
		AddItem(nil, 0, 1, false)

	a.ShowModal("project_form", modal)
}

// validateGitRepo checks if the path is a valid git repository
func validateGitRepo(path string) error {
	// Check if path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist")
	}

	// Check if it's a git repository
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a git repository")
	}

	return nil
}
