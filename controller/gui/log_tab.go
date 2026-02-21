package gui

import (
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
		a.logView.Clear()
	})

	copyBtn := widget.NewButton("Copy to Clipboard", func() {
		a.window.Clipboard().SetContent(a.logView.CopyText())
	})

	toolbar := container.NewHBox(clearBtn, copyBtn)

	return container.NewBorder(nil, toolbar, nil, nil, a.logView)
}
