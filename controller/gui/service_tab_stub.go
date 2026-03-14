//go:build !darwin && !windows && !linux

package gui

import "fyne.io/fyne/v2"

// serviceTab returns nil on unsupported platforms.
func (a *App) serviceTab() fyne.CanvasObject {
	return nil
}
