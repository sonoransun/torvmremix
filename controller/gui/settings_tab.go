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
	// Snapshot original values for dirty tracking.
	origMem := a.cfg.VMMemoryMB
	origCPU := a.cfg.VMCPUs
	origSOCKS := a.cfg.SOCKSPort
	origVerbose := a.cfg.Verbose

	dirty := false
	var settingsTabItem *container.TabItem // set later to update label

	markDirty := func() {
		isDirty := a.cfg.VMMemoryMB != origMem ||
			a.cfg.VMCPUs != origCPU ||
			a.cfg.SOCKSPort != origSOCKS ||
			a.cfg.Verbose != origVerbose
		if isDirty != dirty {
			dirty = isDirty
			if a.tabs != nil && settingsTabItem != nil {
				if dirty {
					settingsTabItem.Text = "Settings *"
				} else {
					settingsTabItem.Text = "Settings"
				}
				a.tabs.Refresh()
			}
		}
	}

	accelLabel := widget.NewLabel("Acceleration: " + a.cfg.Accel)
	accelLabel.TextStyle = fyne.TextStyle{Italic: true}

	memSlider := widget.NewSlider(64, 512)
	memSlider.Step = 16
	memSlider.Value = float64(a.cfg.VMMemoryMB)
	memLabel := widget.NewLabel("VM Memory: " + strconv.Itoa(a.cfg.VMMemoryMB) + " MB")
	memSlider.OnChanged = func(v float64) {
		a.cfg.VMMemoryMB = int(v)
		memLabel.SetText("VM Memory: " + strconv.Itoa(int(v)) + " MB")
		markDirty()
	}

	cpuSlider := widget.NewSlider(1, 8)
	cpuSlider.Step = 1
	cpuSlider.Value = float64(a.cfg.VMCPUs)
	cpuLabel := widget.NewLabel("VM CPUs: " + strconv.Itoa(a.cfg.VMCPUs))
	cpuSlider.OnChanged = func(v float64) {
		a.cfg.VMCPUs = int(v)
		cpuLabel.SetText("VM CPUs: " + strconv.Itoa(int(v)))
		markDirty()
	}

	socksEntry := widget.NewEntry()
	socksEntry.SetText(strconv.Itoa(a.cfg.SOCKSPort))
	socksValidLabel := widget.NewLabel("")
	socksValidLabel.TextStyle = fyne.TextStyle{Italic: true}
	socksEntry.OnChanged = func(s string) {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 65535 {
			socksValidLabel.SetText("Invalid port (1-65535)")
			return
		}
		socksValidLabel.SetText("")
		a.cfg.SOCKSPort = n
		markDirty()
	}

	verboseCheck := widget.NewCheck("Verbose Logging", func(on bool) {
		a.cfg.Verbose = on
		markDirty()
	})
	verboseCheck.Checked = a.cfg.Verbose

	configPathLabel := widget.NewLabel("Config: " + a.configPath)

	saveBtn := widget.NewButton("Save Config", func() {
		a.saveConfig()
		// After save, update original values.
		origMem = a.cfg.VMMemoryMB
		origCPU = a.cfg.VMCPUs
		origSOCKS = a.cfg.SOCKSPort
		origVerbose = a.cfg.Verbose
		markDirty()
	})

	resetBtn := widget.NewButton("Reset to Defaults", func() {
		dialog.ShowConfirm("Reset Settings",
			"Reset all settings to default values? Unsaved changes will be lost.",
			func(ok bool) {
				if !ok {
					return
				}
				a.cfg.VMMemoryMB = 256
				a.cfg.VMCPUs = 2
				a.cfg.SOCKSPort = 9050
				a.cfg.Verbose = false
				memSlider.SetValue(float64(a.cfg.VMMemoryMB))
				cpuSlider.SetValue(float64(a.cfg.VMCPUs))
				socksEntry.SetText(strconv.Itoa(a.cfg.SOCKSPort))
				verboseCheck.SetChecked(a.cfg.Verbose)
				socksValidLabel.SetText("")
				markDirty()
			}, a.window)
	})

	content := container.NewVBox(
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
		socksValidLabel,
		widget.NewSeparator(),
		verboseCheck,
		widget.NewSeparator(),
		configPathLabel,
		container.NewHBox(saveBtn, resetBtn),
		layout.NewSpacer(),
	)

	// Store tab item reference for dirty label updates.
	// The tab item is created in app.go, so we find it after Run() sets up tabs.
	// Use a deferred approach: register a one-time callback.
	go func() {
		// Wait for tabs to be initialized.
		for a.tabs == nil {
			continue
		}
		for _, item := range a.tabs.Items {
			if item.Text == "Settings" || item.Text == "Settings *" {
				settingsTabItem = item
				break
			}
		}
	}()

	return content
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
