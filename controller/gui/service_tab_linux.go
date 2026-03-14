//go:build linux

package gui

import (
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/user/extorvm/controller/internal/systemd"
)

// serviceTab builds the Service tab for Linux systemd management.
func (a *App) serviceTab() fyne.CanvasObject {
	statusLabel := widget.NewLabel("Checking...")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	installBtn := widget.NewButton("Install Service", func() {
		if err := systemd.Install(); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		dialog.ShowInformation("Service", "TorVM service installed successfully.", a.window)
	})

	uninstallBtn := widget.NewButton("Uninstall Service", func() {
		dialog.ShowConfirm("Confirm", "Uninstall the TorVM systemd service?", func(ok bool) {
			if !ok {
				return
			}
			if err := systemd.Uninstall(); err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			a.serviceMode = false
			dialog.ShowInformation("Service", "TorVM service uninstalled.", a.window)
		}, a.window)
	})

	startBtn := widget.NewButton("Start Service", func() {
		if err := systemd.Start(); err != nil {
			dialog.ShowError(err, a.window)
		}
	})

	stopBtn := widget.NewButton("Stop Service", func() {
		if err := systemd.Stop(); err != nil {
			dialog.ShowError(err, a.window)
		}
	})

	restartBtn := widget.NewButton("Restart Service", func() {
		if err := systemd.Restart(); err != nil {
			dialog.ShowError(err, a.window)
		}
	})

	bootCheck := widget.NewCheck("Start on Boot", func(on bool) {
		var err error
		if on {
			err = systemd.Enable()
		} else {
			err = systemd.Disable()
		}
		if err != nil {
			dialog.ShowError(err, a.window)
		}
	})

	viewLogsBtn := widget.NewButton("View Journal Logs", func() {
		text, err := systemd.ReadJournalLogs(100)
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		if text == "" {
			text = "(no log output)"
		}
		d := dialog.NewCustom("Service Logs", "Close",
			container.NewScroll(widget.NewLabel(text)), a.window)
		d.Resize(fyne.NewSize(600, 400))
		d.Show()
	})

	// Update button states based on service status.
	updateUI := func() {
		st := systemd.QueryStatus()

		if !st.Installed {
			statusLabel.SetText("Status: Not Installed")
			startBtn.Disable()
			stopBtn.Disable()
			restartBtn.Disable()
			uninstallBtn.Disable()
			installBtn.Enable()
			bootCheck.Disable()
			bootCheck.SetChecked(false)
			viewLogsBtn.Disable()
		} else if st.Active {
			pidInfo := ""
			if st.PID != "" {
				pidInfo = " (PID " + st.PID + ")"
			}
			statusLabel.SetText("Status: Running" + pidInfo)
			startBtn.Disable()
			stopBtn.Enable()
			restartBtn.Enable()
			uninstallBtn.Disable()
			installBtn.Disable()
			bootCheck.Enable()
			bootCheck.SetChecked(st.Enabled)
			viewLogsBtn.Enable()
			a.serviceMode = true
		} else {
			statusLabel.SetText("Status: Stopped")
			startBtn.Enable()
			stopBtn.Disable()
			restartBtn.Disable()
			uninstallBtn.Enable()
			installBtn.Disable()
			bootCheck.Enable()
			bootCheck.SetChecked(st.Enabled)
			viewLogsBtn.Enable()
			a.serviceMode = true
		}
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
		widget.NewLabelWithStyle("Linux Service Management", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		statusLabel,
		widget.NewSeparator(),
		container.NewHBox(installBtn, uninstallBtn),
		container.NewHBox(startBtn, stopBtn, restartBtn),
		widget.NewSeparator(),
		bootCheck,
		widget.NewSeparator(),
		viewLogsBtn,
		layout.NewSpacer(),
	)
}

func intToStr(n int) string {
	return strconv.Itoa(n)
}
