package gui

import (
	"encoding/json"
	"os"

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
	memLabel := widget.NewLabel("VM Memory: " + intToStr(a.cfg.VMMemoryMB) + " MB")
	memSlider.OnChanged = func(v float64) {
		a.cfg.VMMemoryMB = int(v)
		memLabel.SetText("VM Memory: " + intToStr(int(v)) + " MB")
	}

	socksEntry := widget.NewEntry()
	socksEntry.SetText(intToStr(a.cfg.SOCKSPort))
	socksEntry.OnChanged = func(s string) {
		n := 0
		for _, c := range s {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		if n > 0 {
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

	if err := os.WriteFile(path, data, 0640); err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	dialog.ShowInformation("Saved", "Configuration saved to "+path, a.window)
}
