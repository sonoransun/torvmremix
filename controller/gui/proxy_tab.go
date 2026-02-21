package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// proxyTab builds the upstream Proxy configuration tab.
func (a *App) proxyTab() fyne.CanvasObject {
	addressEntry := widget.NewEntry()
	addressEntry.SetPlaceHolder("host:port")
	addressEntry.SetText(a.cfg.Proxy.Address)
	addressEntry.OnChanged = func(s string) { a.cfg.Proxy.Address = s }

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("username")
	usernameEntry.SetText(a.cfg.Proxy.Username)
	usernameEntry.OnChanged = func(s string) { a.cfg.Proxy.Username = s }

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("password")
	passwordEntry.SetText(a.cfg.Proxy.Password)
	passwordEntry.OnChanged = func(s string) { a.cfg.Proxy.Password = s }

	authFields := container.NewVBox(
		widget.NewLabel("Username:"),
		usernameEntry,
		widget.NewLabel("Password:"),
		passwordEntry,
	)

	addressRow := container.NewVBox(
		widget.NewLabel("Proxy Address:"),
		addressEntry,
	)

	proxyFields := container.NewVBox(addressRow, authFields)
	proxyFields.Hide()

	typeSelect := widget.NewSelect(
		[]string{"None", "HTTP", "HTTPS", "SOCKS5"},
		func(val string) {
			switch val {
			case "None":
				a.cfg.Proxy.Type = ""
				proxyFields.Hide()
			default:
				a.cfg.Proxy.Type = val
				proxyFields.Show()
			}
		},
	)

	// Set initial state.
	switch a.cfg.Proxy.Type {
	case "":
		typeSelect.SetSelected("None")
	default:
		typeSelect.SetSelected(a.cfg.Proxy.Type)
		proxyFields.Show()
	}

	return container.NewVBox(
		widget.NewLabel("Upstream Proxy Type:"),
		typeSelect,
		proxyFields,
		layout.NewSpacer(),
	)
}
