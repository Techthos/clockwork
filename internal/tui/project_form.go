package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/techthos/clockwork/internal/db"
	"github.com/techthos/clockwork/internal/git"
	"github.com/techthos/clockwork/internal/models"
	"github.com/techthos/clockwork/internal/source"
)

// sourceOptions lists the lookup methods in the order they appear in the
// dropdown, alongside the labels shown to the user.
var sourceOptions = source.All()

func sourceLabels() []string {
	labels := make([]string, len(sourceOptions))
	for i, t := range sourceOptions {
		labels[i] = fmt.Sprintf("%s - %s", t, source.Describe(t))
	}
	return labels
}

func sourceIndex(t source.Type) int {
	for i, candidate := range sourceOptions {
		if candidate == t {
			return i
		}
	}
	return 0
}

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
	repoPathField := ""
	repositoryField := ""
	selectedSource := source.Local
	if isEdit {
		nameField = project.Name
		repoPathField = project.GitRepoPath
		repositoryField = project.Repository
		selectedSource = source.Resolve(project)
	}

	form.AddInputField("Name", nameField, 40, nil, func(text string) {
		nameField = text
	})

	// The two locator fields stay visible at all times; only the one matching
	// the selected source is read on save.
	form.AddDropDown("Source", sourceLabels(), sourceIndex(selectedSource), func(option string, optionIndex int) {
		if optionIndex >= 0 && optionIndex < len(sourceOptions) {
			selectedSource = sourceOptions[optionIndex]
		}
	})

	form.AddInputField("Git Repo Path (local)", repoPathField, 60, nil, func(text string) {
		repoPathField = text
	})

	form.AddInputField("Repository (mcp)", repositoryField, 60, nil, func(text string) {
		repositoryField = text
	})

	// Add buttons
	form.AddButton("Save", func() {
		// Validate inputs
		if nameField == "" {
			a.ShowErrorModal("Project name cannot be empty", nil)
			return
		}

		// Keep only the locator that belongs to the selected source, so
		// switching sources does not leave a stale value behind.
		repoPath, repository := "", ""
		switch selectedSource {
		case source.Local:
			if repoPathField == "" {
				a.ShowErrorModal("Source 'local' requires a git repository path", nil)
				return
			}
			if err := git.IsRepo(repoPathField); err != nil {
				a.ShowErrorModal(fmt.Sprintf("Invalid git repository: %v", err), nil)
				return
			}
			repoPath = repoPathField
		case source.MCP:
			if repositoryField == "" {
				a.ShowErrorModal("Source 'mcp' requires a repository identifier (e.g. 'owner/name' or a clone URL)", nil)
				return
			}
			repository = repositoryField
		}

		var err error
		if isEdit {
			sourceType := selectedSource.String()
			_, err = a.store.UpdateProject(project.ID, db.ProjectUpdate{
				Name:        &nameField,
				SourceType:  &sourceType,
				GitRepoPath: &repoPath,
				Repository:  &repository,
			})
		} else {
			_, err = a.store.CreateProject(db.ProjectInput{
				Name:        nameField,
				SourceType:  selectedSource.String(),
				GitRepoPath: repoPath,
				Repository:  repository,
			})
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
			AddItem(form, 16, 1, true).
			AddItem(nil, 0, 1, false), 100, 1, true).
		AddItem(nil, 0, 1, false)

	a.ShowModal("project_form", modal)
}
