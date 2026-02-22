package gui

import (
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/user/extorvm/controller/internal/launchd"
	"github.com/user/extorvm/controller/internal/lifecycle"
)

// statusTab builds the Status tab content.
func (a *App) statusTab() fyne.CanvasObject {
	a.statusLight = NewStatusLight()
	a.stateLabel = widget.NewLabel("Stopped")
	a.stateLabel.TextStyle = fyne.TextStyle{Bold: true}

	modeTxt := "Mode: Direct"
	if a.serviceMode {
		modeTxt = "Mode: Service"
	}
	a.modeLabel = widget.NewLabel(modeTxt)
	a.modeLabel.TextStyle = fyne.TextStyle{Italic: true}

	startBtn := widget.NewButton("Start", func() { a.startVM() })
	stopBtn := widget.NewButton("Stop", func() { a.stopVM() })

	statusRow := container.NewHBox(a.statusLight, a.stateLabel)
	buttonRow := container.NewHBox(startBtn, stopBtn)

	accelLabel := widget.NewLabel("Acceleration: " + a.cfg.Accel)
	memLabel := widget.NewLabel("VM Memory: " + strconv.Itoa(a.cfg.VMMemoryMB) + " MB")
	hostIPLabel := widget.NewLabel("Host IP: " + a.cfg.HostIP)
	vmIPLabel := widget.NewLabel("VM IP: " + a.cfg.VMIP)

	info := container.NewVBox(
		accelLabel,
		memLabel,
		hostIPLabel,
		vmIPLabel,
	)

	// In service mode, poll launchd for status display.
	if a.serviceMode {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if !a.serviceMode {
					return
				}
				a.pollServiceStatus()
			}
		}()
		// Initial poll.
		a.pollServiceStatus()
	}

	return container.NewVBox(
		a.modeLabel,
		statusRow,
		buttonRow,
		widget.NewSeparator(),
		info,
		layout.NewSpacer(),
	)
}

// updateStatus is called by the observer to update the status tab.
func (a *App) updateStatus(_, to lifecycle.State) {
	a.statusLight.SetState(to)
	a.stateLabel.SetText(to.String())
}

// pollServiceStatus queries launchd and updates the status widgets.
func (a *App) pollServiceStatus() {
	st := launchd.QueryStatus()
	if st.Running {
		a.statusLight.SetState(lifecycle.StateRunning)
		a.stateLabel.SetText("Running (Service)")
	} else if st.Installed {
		a.statusLight.SetState(lifecycle.StateInit)
		a.stateLabel.SetText("Stopped (Service)")
	} else {
		a.serviceMode = false
		a.modeLabel.SetText("Mode: Direct")
		a.statusLight.SetState(lifecycle.StateInit)
		a.stateLabel.SetText("Stopped")
	}
}
