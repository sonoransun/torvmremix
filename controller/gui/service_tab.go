//go:build darwin

package gui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/user/extorvm/controller/internal/launchd"
)

// serviceTab builds the Service tab for macOS launchd management.
func (a *App) serviceTab() fyne.CanvasObject {
	statusLabel := widget.NewLabel("Checking...")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	installBtn := widget.NewButton("Install Service", func() {
		if err := launchd.Install(false); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		dialog.ShowInformation("Service", "Service installed successfully.", a.window)
	})

	uninstallBtn := widget.NewButton("Uninstall Service", func() {
		if err := launchd.Uninstall(); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		a.serviceMode = false
		dialog.ShowInformation("Service", "Service uninstalled.", a.window)
	})

	startBtn := widget.NewButton("Start Service", func() {
		if err := launchd.Start(); err != nil {
			dialog.ShowError(err, a.window)
		}
	})

	stopBtn := widget.NewButton("Stop Service", func() {
		if err := launchd.Stop(); err != nil {
			dialog.ShowError(err, a.window)
		}
	})

	bootCheck := widget.NewCheck("Start on Boot", func(on bool) {
		if err := launchd.SetRunAtLoad(on); err != nil {
			dialog.ShowError(err, a.window)
		}
	})

	viewLogsBtn := widget.NewButton("View Service Logs", func() {
		text, err := launchd.ReadLog(100)
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

	// Update button/checkbox states based on service status.
	updateUI := func() {
		st := launchd.QueryStatus()

		if !st.Installed {
			statusLabel.SetText("Status: Not Installed")
			startBtn.Disable()
			stopBtn.Disable()
			uninstallBtn.Disable()
			installBtn.Enable()
			bootCheck.Disable()
			bootCheck.SetChecked(false)
			viewLogsBtn.Disable()
		} else if st.Running {
			statusLabel.SetText("Status: Running (PID " + intToStr(st.PID) + ")")
			startBtn.Disable()
			stopBtn.Enable()
			uninstallBtn.Disable()
			installBtn.Disable()
			bootCheck.Enable()
			bootCheck.SetChecked(st.RunAtLoad)
			viewLogsBtn.Enable()
			a.serviceMode = true
		} else {
			statusLabel.SetText("Status: Stopped")
			startBtn.Enable()
			stopBtn.Disable()
			uninstallBtn.Enable()
			installBtn.Disable()
			bootCheck.Enable()
			bootCheck.SetChecked(st.RunAtLoad)
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
		widget.NewLabelWithStyle("macOS Service Management", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		statusLabel,
		widget.NewSeparator(),
		container.NewHBox(installBtn, uninstallBtn),
		container.NewHBox(startBtn, stopBtn),
		widget.NewSeparator(),
		bootCheck,
		widget.NewSeparator(),
		viewLogsBtn,
		layout.NewSpacer(),
	)
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	v := n
	if v < 0 {
		v = -v
	}
	for v > 0 {
		s = string(rune('0'+v%10)) + s
		v /= 10
	}
	if n < 0 {
		s = "-" + s
	}
	return s
}
