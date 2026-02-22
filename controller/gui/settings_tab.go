package gui

import (
	"encoding/json"
	"os"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// settingsTab builds the Settings tab.
func (a *App) settingsTab() fyne.CanvasObject {
	accelLabel := widget.NewLabel("Acceleration: " + a.cfg.Accel)
	accelLabel.TextStyle = fyne.TextStyle{Italic: true}

	memSlider := widget.NewSlider(64, 512)
	memSlider.Step = 16
	memSlider.Value = float64(a.cfg.VMMemoryMB)
	memLabel := widget.NewLabel("VM Memory: " + strconv.Itoa(a.cfg.VMMemoryMB) + " MB")
	memSlider.OnChanged = func(v float64) {
		a.cfg.VMMemoryMB = int(v)
		memLabel.SetText("VM Memory: " + strconv.Itoa(int(v)) + " MB")
	}

	cpuSlider := widget.NewSlider(1, 8)
	cpuSlider.Step = 1
	cpuSlider.Value = float64(a.cfg.VMCPUs)
	cpuLabel := widget.NewLabel("VM CPUs: " + strconv.Itoa(a.cfg.VMCPUs))
	cpuSlider.OnChanged = func(v float64) {
		a.cfg.VMCPUs = int(v)
		cpuLabel.SetText("VM CPUs: " + strconv.Itoa(int(v)))
	}

	socksEntry := widget.NewEntry()
	socksEntry.SetText(strconv.Itoa(a.cfg.SOCKSPort))
	socksEntry.OnChanged = func(s string) {
		n, err := strconv.Atoi(s)
		if err == nil && n >= 1 && n <= 65535 {
			a.cfg.SOCKSPort = n
		}
	}

	verboseCheck := widget.NewCheck("Verbose Logging", func(on bool) {
		a.cfg.Verbose = on
	})
	verboseCheck.Checked = a.cfg.Verbose

	configPathLabel := widget.NewLabel("Config: " + a.configPath)

	saveBtn := widget.NewButton("Save Config", func() {
		a.saveConfig()
	})

	return container.NewVBox(
		accelLabel,
		widget.NewSeparator(),
		memLabel,
		memSlider,
		widget.NewSeparator(),
		cpuLabel,
		cpuSlider,
		widget.NewSeparator(),
		widget.NewLabel("SOCKS Port:"),
		socksEntry,
		widget.NewSeparator(),
		verboseCheck,
		widget.NewSeparator(),
		configPathLabel,
		saveBtn,
		layout.NewSpacer(),
	)
}

func (a *App) saveConfig() {
	path := a.configPath
	if path == "" {
		path = "torvm.json"
	}

	data, err := json.MarshalIndent(a.cfg, "", "  ")
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	dialog.ShowInformation("Saved", "Configuration saved to "+path, a.window)
}
