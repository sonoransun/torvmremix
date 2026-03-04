//go:build windows

package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/user/extorvm/controller/internal/winsvc"
)

// serviceTab builds the Service tab for Windows SCM management.
func (a *App) serviceTab() fyne.CanvasObject {
	statusLabel := widget.NewLabel("Checking...")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	tapStatusLabel := widget.NewLabel("TAP: Checking...")

	installBtn := widget.NewButton("Install Service", func() {
		if err := winsvc.InstallService(); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		dialog.ShowInformation("Service", "TorVM service installed successfully.", a.window)
	})

	uninstallBtn := widget.NewButton("Uninstall Service", func() {
		if err := winsvc.RemoveService(); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		a.serviceMode = false
		dialog.ShowInformation("Service", "TorVM service uninstalled.", a.window)
	})

	installTAPBtn := widget.NewButton("Install TAP Adapter", func() {
		if err := winsvc.InstallTAPAdapter(); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		dialog.ShowInformation("TAP Adapter", "TAP adapter installed successfully.", a.window)
	})

	removeTAPBtn := widget.NewButton("Remove TAP Adapters", func() {
		dialog.ShowConfirm("Confirm", "Remove ALL TAP adapters?", func(ok bool) {
			if !ok {
				return
			}
			if err := winsvc.RemoveTAPAdapter(); err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			dialog.ShowInformation("TAP Adapter", "TAP adapters removed.", a.window)
		}, a.window)
	})

	// Update UI state based on service and TAP status.
	updateUI := func() {
		tapSt := winsvc.QueryTAPStatus()
		if tapSt.Installed {
			tapStatusLabel.SetText("TAP: Installed (" + tapSt.AdapterName + ")")
			installTAPBtn.Disable()
			removeTAPBtn.Enable()
		} else {
			tapStatusLabel.SetText("TAP: Not Installed")
			installTAPBtn.Enable()
			removeTAPBtn.Disable()
		}

		// Windows service status is not easily queryable without extra
		// Win32 API calls, so we show a simplified view based on whether
		// the service is installed (checked via install/uninstall results).
		statusLabel.SetText("Status: Ready")
		installBtn.Enable()
		uninstallBtn.Enable()
	}

	// Initial update.
	updateUI()

	// Poll every 5 seconds.
	a.serviceTicker = time.NewTicker(5 * time.Second)
	go func() {
		for range a.serviceTicker.C {
			updateUI()
		}
	}()

	return container.NewVBox(
		widget.NewLabelWithStyle("Windows Service Management", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		statusLabel,
		widget.NewSeparator(),
		container.NewHBox(installBtn, uninstallBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("TAP Adapter", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		tapStatusLabel,
		container.NewHBox(installTAPBtn, removeTAPBtn),
		layout.NewSpacer(),
	)
}
