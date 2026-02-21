package gui

import (
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
	memLabel := widget.NewLabel("VM Memory: " + fyne.CurrentApp().Metadata().Name)
	hostIPLabel := widget.NewLabel("Host IP: " + a.cfg.HostIP)
	vmIPLabel := widget.NewLabel("VM IP: " + a.cfg.VMIP)

	// Use a simple label for memory since Metadata may not be set.
	memLabel.SetText("VM Memory: " + intToStr(a.cfg.VMMemoryMB) + " MB")

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

func intToStr(n int) string {
	// Simple int to string without importing strconv in this file.
	if n == 0 {
		return "0"
	}
	s := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
