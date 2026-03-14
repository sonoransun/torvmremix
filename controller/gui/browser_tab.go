package gui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/user/extorvm/controller/internal/lifecycle"
	"github.com/user/extorvm/controller/internal/secwatch"
)

// browserTab builds the Browser VM tab with status, controls, and
// security monitor indicators.
func (a *App) browserTab() fyne.CanvasObject {
	statusLight := NewStatusLight()
	stateLabel := widget.NewLabel("Idle")
	stateLabel.TextStyle = fyne.TextStyle{Bold: true}

	launchBtn := widget.NewButton("Launch Browser", func() {
		a.launchBrowser()
	})
	stopBtn := widget.NewButton("Stop Browser", func() {
		a.stopBrowser()
	})

	vncPort := 5900 + a.cfg.Browser.VNCDisplay
	vncLabel := widget.NewLabel(fmt.Sprintf("VNC: vnc://127.0.0.1:%d", vncPort))
	if a.cfg.Browser.VNCDisplay == 0 {
		vncLabel.SetText("VNC: disabled (headless)")
	}

	// Security monitor status labels.
	secHeader := widget.NewLabel("Security Monitor")
	secHeader.TextStyle = fyne.TextStyle{Bold: true}

	canaryLabel := widget.NewLabel("Canaries: --")
	honeyLabel := widget.NewLabel("Honey Tokens: --")
	seccompLabel := widget.NewLabel("Seccomp: --")
	integrityLabel := widget.NewLabel("File Integrity: --")
	lastEventLabel := widget.NewLabel("Last Event: none")

	// Security event log.
	var eventMu sync.Mutex
	var events []secwatch.SecurityEvent

	eventList := widget.NewList(
		func() int {
			eventMu.Lock()
			defer eventMu.Unlock()
			return len(events)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("placeholder security event text")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			eventMu.Lock()
			defer eventMu.Unlock()
			label := obj.(*widget.Label)
			if id < len(events) {
				ev := events[id]
				ts := ev.Time().Format("15:04:05")
				label.SetText(fmt.Sprintf("[%s] %s %s: %s", ts, ev.Severity, ev.Type, ev.Detail))
			}
		},
	)

	// Map browser lifecycle states to the StatusLight color states.
	browserStateToLifecycle := func(bs lifecycle.BrowserState) lifecycle.State {
		switch bs {
		case lifecycle.BrowserRunning:
			return lifecycle.StateRunning
		case lifecycle.BrowserCompromised:
			return lifecycle.StateFailed
		case lifecycle.BrowserIdle:
			return lifecycle.StateInit
		default:
			return lifecycle.StateWaitBootstrap // amber for transitional
		}
	}

	if a.browserEngine != nil {
		a.browserEngine.OnStateChange(func(_, to lifecycle.BrowserState) {
			statusLight.SetState(browserStateToLifecycle(to))
			stateLabel.SetText(to.String())

			switch to {
			case lifecycle.BrowserRunning:
				canaryLabel.SetText("Canaries: OK")
				honeyLabel.SetText("Honey Tokens: Untouched")
				seccompLabel.SetText("Seccomp: Active")
				integrityLabel.SetText("File Integrity: OK")
			case lifecycle.BrowserIdle:
				canaryLabel.SetText("Canaries: --")
				honeyLabel.SetText("Honey Tokens: --")
				seccompLabel.SetText("Seccomp: --")
				integrityLabel.SetText("File Integrity: --")
			case lifecycle.BrowserCompromised:
				lastEventLabel.SetText("COMPROMISE DETECTED - see event log")
			}
		})

		a.browserEngine.SecMonitor.OnEvent(func(ev secwatch.SecurityEvent) {
			eventMu.Lock()
			events = append(events, ev)
			if len(events) > 200 {
				events = events[len(events)-200:]
			}
			eventMu.Unlock()
			eventList.Refresh()

			ts := time.Unix(ev.Timestamp, 0).Format("15:04:05")
			lastEventLabel.SetText(fmt.Sprintf("Last Event: [%s] %s", ts, ev.Type))

			switch ev.Type {
			case "canary_violation":
				canaryLabel.SetText("Canaries: VIOLATION DETECTED")
			case "honey_token_access":
				honeyLabel.SetText("Honey Tokens: ACCESSED - " + ev.Detail)
			case "seccomp_kill":
				seccompLabel.SetText(fmt.Sprintf("Seccomp: Violation PID %d killed", ev.PID))
			case "file_tamper":
				integrityLabel.SetText("File Integrity: TAMPERED - " + ev.Detail)
			}

			if ev.IsCritical() {
				dialog.ShowInformation("Security Alert",
					fmt.Sprintf("Browser VM security event: %s\n%s", ev.Type, ev.Detail),
					a.window)
			}
		})
	}

	eventListBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(600, 150)), eventList)

	content := container.NewVBox(
		widget.NewLabel("Browser VM"),
		widget.NewSeparator(),
		container.NewHBox(statusLight, stateLabel),
		container.NewHBox(launchBtn, stopBtn),
		vncLabel,
		widget.NewSeparator(),
		secHeader,
		canaryLabel,
		honeyLabel,
		seccompLabel,
		integrityLabel,
		lastEventLabel,
		widget.NewSeparator(),
		widget.NewLabel("Security Event Log"),
		eventListBox,
		layout.NewSpacer(),
	)

	return container.NewScroll(content)
}

// launchBrowser starts the browser VM in the background.
func (a *App) launchBrowser() {
	if a.browserEngine == nil {
		dialog.ShowError(fmt.Errorf("browser VM not configured"), a.window)
		return
	}
	if a.browserEngine.State() != lifecycle.BrowserIdle {
		return
	}
	if a.engine.State() != lifecycle.StateRunning {
		dialog.ShowInformation("Not Ready",
			"TorVM must be running before launching the browser.",
			a.window)
		return
	}

	errCh := a.browserEngine.Start(context.Background())
	go func() {
		if err := <-errCh; err != nil {
			a.logger.Error("browser VM: %v", err)
		}
	}()
}

// stopBrowser stops the browser VM.
func (a *App) stopBrowser() {
	if a.browserEngine != nil {
		a.browserEngine.Stop()
	}
}

