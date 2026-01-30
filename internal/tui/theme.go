package tui

import "github.com/gdamore/tcell/v2"

// Color constants for the TUI
var (
	// Primary colors
	ColorPrimary   = tcell.ColorDodgerBlue
	ColorSecondary = tcell.ColorDarkCyan
	ColorAccent    = tcell.ColorOrange

	// Status colors
	ColorSuccess = tcell.ColorGreen
	ColorWarning = tcell.ColorYellow
	ColorError   = tcell.ColorRed
	ColorInfo    = tcell.ColorSkyblue

	// UI element colors
	ColorBorder     = tcell.ColorGray
	ColorTitle      = tcell.ColorWhite
	ColorText       = tcell.ColorWhite
	ColorBackground = tcell.ColorDefault

	// Table colors
	ColorTableHeader = ColorPrimary
	ColorTableText   = ColorText

	// Invoiced status
	ColorInvoiced   = ColorSuccess
	ColorUninvoiced = ColorWarning
)
