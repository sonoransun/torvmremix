package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// logTab builds the Logs tab with live log viewing.
func (a *App) logTab() fyne.CanvasObject {
	a.logView = NewLogView(a.ring)

	// Register callback so new lines trigger a UI refresh.
	a.ring.OnLine(func(_ string) {
		a.logView.Refresh()
	})

	clearBtn := widget.NewButton("Clear", func() {
		// Replace ring with a fresh one of the same capacity.
		// The old ring is still connected as a writer but lines
		// are discarded visually.
		a.logView.mu.Lock()
		a.logView.snapshot = nil
		a.logView.mu.Unlock()
		a.logView.list.Refresh()
	})

	copyBtn := widget.NewButton("Copy to Clipboard", func() {
		a.logView.mu.Lock()
		text := strings.Join(a.logView.snapshot, "\n")
		a.logView.mu.Unlock()
		a.window.Clipboard().SetContent(text)
	})

	toolbar := container.NewHBox(clearBtn, copyBtn)

	return container.NewBorder(nil, toolbar, nil, nil, a.logView)
}
