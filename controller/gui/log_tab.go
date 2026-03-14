package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// logTab builds the Logs tab with live log viewing, search, and filtering.
func (a *App) logTab() fyne.CanvasObject {
	a.logView = NewLogView(a.ring)

	// Register callback so new lines trigger a UI refresh.
	a.ring.OnLine(func(_ string) {
		a.logView.Refresh()
	})

	// Search entry for text filtering.
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Filter logs...")
	searchEntry.OnChanged = func(text string) {
		a.logView.SetFilter(text)
	}

	// Level filter dropdown.
	levelSelect := widget.NewSelect([]string{"All", "ERROR", "INFO", "DEBUG"}, func(level string) {
		a.logView.SetLevelFilter(level)
	})
	levelSelect.SetSelected("All")

	clearBtn := widget.NewButton("Clear", func() {
		a.logView.Clear()
	})

	copyBtn := widget.NewButton("Copy", func() {
		a.window.Clipboard().SetContent(a.logView.CopyText())
	})

	exportBtn := widget.NewButton("Export", func() {
		a.exportLogs()
	})

	filterRow := container.NewBorder(nil, nil, widget.NewLabel("Level:"), nil, searchEntry)
	toolbar := container.NewHBox(levelSelect, clearBtn, copyBtn, exportBtn)

	top := container.NewVBox(filterRow, toolbar)

	return container.NewBorder(top, nil, nil, nil, a.logView)
}

// exportLogs writes the current filtered log view to a timestamped file.
func (a *App) exportLogs() {
	text := a.logView.CopyText()
	if text == "" {
		dialog.ShowInformation("Export", "No log lines to export.", a.window)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	filename := fmt.Sprintf("torvm-logs-%s.txt", time.Now().Format("20060102-150405"))
	path := filepath.Join(home, filename)

	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	dialog.ShowInformation("Exported", "Logs saved to "+path, a.window)
}
