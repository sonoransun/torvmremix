package gui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed tray_icon.png
var trayIconBytes []byte

var trayIconResource = &fyne.StaticResource{
	StaticName:    "tray_icon.png",
	StaticContent: trayIconBytes,
}
