package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/techthos/clockwork/internal/db"
)

// App represents the main TUI application
type App struct {
	app   *tview.Application
	pages *tview.Pages
	store *db.Store

	// Current state
	currentProjectID string // Used when filtering entries by project
}

// New creates a new TUI application instance
func New(store *db.Store) *App {
	tuiApp := &App{
		app:   tview.NewApplication(),
		pages: tview.NewPages(),
		store: store,
	}

	// Set up the application
	tuiApp.app.SetRoot(tuiApp.pages, true)

	return tuiApp
}

// Run starts the TUI application
func (a *App) Run() error {
	// Show the projects view as the default page
	a.ShowProjectsView()
	return a.app.Run()
}

// Stop stops the TUI application
func (a *App) Stop() {
	a.app.Stop()
}

// ShowProjectsView displays the projects list view
func (a *App) ShowProjectsView() {
	view := a.createProjectsView()
	a.pages.AddAndSwitchToPage("projects", view, true)
}

// ShowEntriesView displays the entries view for a specific project
func (a *App) ShowEntriesView(projectID string) {
	a.currentProjectID = projectID
	view := a.createEntriesView(projectID)
	a.pages.AddAndSwitchToPage("entries", view, true)
}

// ShowStatsView displays the statistics view
func (a *App) ShowStatsView(projectID string, filterOptions *FilterOptions) {
	view := a.createStatsView(projectID, filterOptions)
	a.pages.AddAndSwitchToPage("stats", view, true)
}

// ShowModal displays a modal on top of the current page
func (a *App) ShowModal(name string, modal tview.Primitive) {
	a.pages.AddPage(name, modal, true, true)
}

// HideModal removes a modal from the page stack
func (a *App) HideModal(name string) {
	a.pages.RemovePage(name)
}

// SetupGlobalKeyBindings sets up global keyboard shortcuts
func (a *App) SetupGlobalKeyBindings(view *tview.Box) {
	view.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC, tcell.KeyCtrlQ:
			a.Stop()
			return nil
		}
		return event
	})
}
