package gui

import (
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// bridgesTab builds the Bridges configuration tab.
func (a *App) bridgesTab() fyne.CanvasObject {
	useBridges := widget.NewCheck("Use Bridges", func(on bool) {
		a.cfg.Bridge.UseBridges = on
	})
	useBridges.Checked = a.cfg.Bridge.UseBridges

	transportSelect := widget.NewSelect(
		[]string{"none", "obfs4", "meek-azure", "snowflake"},
		func(val string) {
			a.cfg.Bridge.Transport = val
		},
	)
	if a.cfg.Bridge.Transport != "" {
		transportSelect.SetSelected(a.cfg.Bridge.Transport)
	} else {
		transportSelect.SetSelected("none")
	}

	bridgeLines := widget.NewMultiLineEntry()
	bridgeLines.SetPlaceHolder("Paste bridge lines here, one per line...")
	bridgeLines.SetMinRowsVisible(6)
	bridgeLines.SetText(strings.Join(a.cfg.Bridge.Bridges, "\n"))
	bridgeLines.OnChanged = func(text string) {
		lines := strings.Split(text, "\n")
		var filtered []string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				filtered = append(filtered, l)
			}
		}
		a.cfg.Bridge.Bridges = filtered
	}

	getBridgesURL, _ := url.Parse("https://bridges.torproject.org")
	getBridges := widget.NewHyperlink("Get Bridges from torproject.org", getBridgesURL)

	return container.NewVBox(
		useBridges,
		widget.NewLabel("Transport:"),
		transportSelect,
		widget.NewLabel("Bridge Lines:"),
		bridgeLines,
		getBridges,
		layout.NewSpacer(),
	)
}
