package gui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/user/extorvm/controller/internal/lifecycle"
)

// statusTab builds the Status tab content.
func (a *App) statusTab() fyne.CanvasObject {
	a.statusLight = NewStatusLight()
	a.stateLabel = widget.NewLabel("Stopped")
	a.stateLabel.TextStyle = fyne.TextStyle{Bold: true}

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

	return container.NewVBox(
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
