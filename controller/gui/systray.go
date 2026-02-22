package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/user/extorvm/controller/internal/lifecycle"
)

// setupSystemTray configures the system tray icon and menu.
func (a *App) setupSystemTray() {
	if drv, ok := a.fyneApp.(desktop.App); ok {
		menu := a.buildTrayMenu()
		drv.SetSystemTrayMenu(menu)
		drv.SetSystemTrayIcon(trayIconResource)
	}
}

// buildTrayMenu creates the system tray menu with current state.
func (a *App) buildTrayMenu() *fyne.Menu {
	showItem := fyne.NewMenuItem("Show Window", func() {
		a.window.Show()
		a.window.RequestFocus()
	})

	var toggleItem *fyne.MenuItem
	if a.engine.State() == lifecycle.StateRunning {
		toggleItem = fyne.NewMenuItem("Stop TorVM", func() {
			a.stopVM()
		})
	} else if a.cancel != nil {
		// Transitional state (starting up / shutting down).
		toggleItem = fyne.NewMenuItem("TorVM Busy...", nil)
		toggleItem.Disabled = true
	} else {
		toggleItem = fyne.NewMenuItem("Start TorVM", func() {
			a.startVM()
		})
	}

	quitItem := fyne.NewMenuItem("Quit", func() {
		a.doQuit()
	})

	return fyne.NewMenu("TorVM", showItem, toggleItem, fyne.NewMenuItemSeparator(), quitItem)
}

// refreshTrayMenu rebuilds the tray menu to reflect current state.
func (a *App) refreshTrayMenu() {
	if drv, ok := a.fyneApp.(desktop.App); ok {
		drv.SetSystemTrayMenu(a.buildTrayMenu())
	}
}

// doQuit performs a clean quit: stop VM if running, then exit.
func (a *App) doQuit() {
	if a.serviceTicker != nil {
		a.serviceTicker.Stop()
	}
	if a.cancel != nil {
		a.cancel()
	}
	a.fyneApp.Quit()
}
