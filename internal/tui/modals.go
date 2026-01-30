package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowErrorModal displays an error message in a modal dialog
func (a *App) ShowErrorModal(message string, onClose func()) {
	modal := tview.NewModal().
		SetText("Error: " + message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.HideModal("error")
			if onClose != nil {
				onClose()
			}
		})

	modal.SetBackgroundColor(tcell.ColorDefault)
	modal.SetBorderColor(ColorError)

	a.ShowModal("error", modal)
}

// ShowInfoModal displays an informational message in a modal dialog
func (a *App) ShowInfoModal(message string, onClose func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.HideModal("info")
			if onClose != nil {
				onClose()
			}
		})

	modal.SetBackgroundColor(tcell.ColorDefault)
	modal.SetBorderColor(ColorInfo)

	a.ShowModal("info", modal)
}

// ShowConfirmModal displays a confirmation dialog
func (a *App) ShowConfirmModal(message string, onConfirm, onCancel func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			a.HideModal("confirm")
			if buttonIndex == 0 && onConfirm != nil {
				onConfirm()
			} else if buttonIndex == 1 && onCancel != nil {
				onCancel()
			}
		})

	modal.SetBackgroundColor(tcell.ColorDefault)
	modal.SetBorderColor(ColorWarning)

	a.ShowModal("confirm", modal)
}
