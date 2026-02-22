package gui

import (
	"context"
	"time"

	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"fyne.io/fyne/v2"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/launchd"
	"github.com/user/extorvm/controller/internal/lifecycle"
	"github.com/user/extorvm/controller/internal/logging"
)

// App is the Fyne-based TorVM GUI application.
type App struct {
	fyneApp fyne.App
	window  fyne.Window
	engine  *lifecycle.Engine
	logger  *logging.Logger
	ring    *logging.RingWriter
	cfg     *config.Config

	configPath    string
	cancel        context.CancelFunc
	serviceMode   bool
	serviceTicker *time.Ticker

	// Widgets updated by observers.
	statusLight *StatusLight
	stateLabel  *widget.Label
	logView     *LogView
	modeLabel   *widget.Label
}

// New creates a GUI application.
func New(cfg *config.Config, engine *lifecycle.Engine, logger *logging.Logger, ring *logging.RingWriter, configPath string) *App {
	return &App{
		cfg:        cfg,
		engine:     engine,
		logger:     logger,
		ring:       ring,
		configPath: configPath,
	}
}

// Run creates the window and starts the Fyne event loop. Blocks until exit.
func (a *App) Run() {
	a.fyneApp = fyneapp.New()
	a.window = a.fyneApp.NewWindow("TorVM")
	a.window.Resize(fyne.NewSize(640, 480))

	// Auto-detect service mode: if service is installed, default to service mode.
	st := launchd.QueryStatus()
	a.serviceMode = st.Installed

	// Register lifecycle observer for UI updates.
	a.engine.OnStateChange(func(from, to lifecycle.State) {
		a.updateStatus(from, to)
		a.refreshTrayMenu()
	})

	tabs := container.NewAppTabs(
		container.NewTabItem("Status", a.statusTab()),
		container.NewTabItem("Bridges", a.bridgesTab()),
		container.NewTabItem("Proxy", a.proxyTab()),
		container.NewTabItem("Settings", a.settingsTab()),
		container.NewTabItem("Logs", a.logTab()),
	)

	// Conditionally add Service tab (macOS only — returns nil on other platforms).
	if svcTab := a.serviceTab(); svcTab != nil {
		tabs.Append(container.NewTabItem("Service", svcTab))
	}

	a.window.SetContent(tabs)

	// Minimize to tray on close instead of quitting.
	a.window.SetCloseIntercept(func() {
		a.window.Hide()
	})

	a.setupSystemTray()
	a.window.ShowAndRun()
}

// startVM begins the lifecycle engine in the background,
// or starts the launchd service if in service mode.
func (a *App) startVM() {
	if a.serviceMode {
		if err := launchd.Start(); err != nil {
			a.logger.Error("service start: %v", err)
			dialog.ShowError(err, a.window)
		}
		return
	}

	if a.cancel != nil {
		// Already running.
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	errCh := a.engine.Start(ctx)

	// Watch for completion in the background.
	go func() {
		err := <-errCh
		a.cancel = nil
		a.refreshTrayMenu()
		if err != nil {
			a.logger.Error("lifecycle error: %v", err)
			a.window.Canvas().Content().Refresh()
			dialog.ShowError(err, a.window)
		}
	}()
}

// stopVM signals the lifecycle engine to shut down,
// or stops the launchd service if in service mode.
func (a *App) stopVM() {
	if a.serviceMode {
		if err := launchd.Stop(); err != nil {
			a.logger.Error("service stop: %v", err)
			dialog.ShowError(err, a.window)
		}
		return
	}

	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
}
