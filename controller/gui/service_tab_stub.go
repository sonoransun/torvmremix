//go:build !darwin && !windows

package gui

import "fyne.io/fyne/v2"

// serviceTab returns nil on non-macOS platforms.
func (a *App) serviceTab() fyne.CanvasObject {
	return nil
}
