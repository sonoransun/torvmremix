package gui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/user/extorvm/controller/internal/lifecycle"
	"github.com/user/extorvm/controller/internal/tor"
)

// circuitEntry holds data for a single circuit list item.
type circuitEntry struct {
	info     tor.CircuitInfo
	resolved []tor.RelayInfo
}

// circuitsTab builds the Circuits visualization tab.
func (a *App) circuitsTab() fyne.CanvasObject {
	globe := NewGlobeWidget()

	var mu sync.Mutex
	var circuits []circuitEntry
	var selectedIdx int = -1
	var autoRefresh bool
	var ticker *time.Ticker
	var tickerDone chan struct{}

	// Relay location cache.
	relayCache := make(map[string]*tor.RelayInfo)

	countLabel := widget.NewLabel("Circuits: 0")

	circuitList := widget.NewList(
		func() int {
			mu.Lock()
			defer mu.Unlock()
			return len(circuits)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("placeholder circuit")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			mu.Lock()
			defer mu.Unlock()
			label := obj.(*widget.Label)
			if id < len(circuits) {
				c := circuits[id]
				pathStr := formatCircuitPath(c)
				label.SetText(fmt.Sprintf("#%s %s %s (%s)", c.info.ID, c.info.Status, pathStr, c.info.Purpose))
			}
		},
	)

	// resolveCircuit resolves relay locations for a circuit, using the cache.
	resolveCircuit := func(ci tor.CircuitInfo) []tor.RelayInfo {
		var resolved []tor.RelayInfo
		for i, entry := range ci.Path {
			fp, nick := tor.ParseRelayPath(entry)

			// Check cache first.
			if cached, ok := relayCache[fp]; ok {
				ri := *cached
				ri.Role = relayRole(i, len(ci.Path))
				resolved = append(resolved, ri)
				continue
			}

			ri := tor.RelayInfo{
				Fingerprint: fp,
				Nickname:    nick,
			}
			ri.Role = relayRole(i, len(ci.Path))

			// Try to resolve via Tor control.
			if a.engine.TorControl != nil {
				if full, err := a.engine.TorControl.ResolveRelay(entry); err == nil {
					ri = *full
					ri.Role = relayRole(i, len(ci.Path))
				}
			}

			relayCache[fp] = &ri
			resolved = append(resolved, ri)
		}
		return resolved
	}

	// updateGlobe pushes circuit data to the globe widget.
	updateGlobe := func() {
		mu.Lock()
		defer mu.Unlock()

		var nodes []GlobeNode
		var paths []GlobePath
		seen := make(map[string]bool)

		for idx, ce := range circuits {
			var pathNodes []GlobeNode
			for _, ri := range ce.resolved {
				gn := GlobeNode{
					Lat:         ri.Latitude,
					Lon:         ri.Longitude,
					Fingerprint: ri.Fingerprint,
					Nickname:    ri.Nickname,
					Country:     ri.CountryCode,
					Role:        ri.Role,
				}
				pathNodes = append(pathNodes, gn)
				if !seen[ri.Fingerprint] {
					seen[ri.Fingerprint] = true
					nodes = append(nodes, gn)
				}
			}
			if len(pathNodes) > 0 {
				paths = append(paths, GlobePath{
					Nodes:     pathNodes,
					CircuitID: ce.info.ID,
					Selected:  idx == selectedIdx,
				})
			}
		}

		globe.SetData(nodes, paths)
	}

	// pollCircuits fetches current circuits and updates the UI.
	pollCircuits := func() {
		if a.engine.TorControl == nil || a.engine.State() != lifecycle.StateRunning {
			return
		}

		circs, err := a.engine.TorControl.GetCircuits()
		if err != nil {
			a.logger.Error("circuits poll: %v", err)
			return
		}

		mu.Lock()
		circuits = circuits[:0]
		for _, ci := range circs {
			if ci.Status != "BUILT" && ci.Status != "EXTENDED" {
				continue
			}
			resolved := resolveCircuit(ci)
			circuits = append(circuits, circuitEntry{info: ci, resolved: resolved})
		}
		mu.Unlock()

		countLabel.SetText(fmt.Sprintf("Circuits: %d", len(circuits)))
		circuitList.Refresh()
		updateGlobe()
	}

	// Circuit list selection.
	circuitList.OnSelected = func(id widget.ListItemID) {
		mu.Lock()
		selectedIdx = id
		mu.Unlock()
		updateGlobe()
	}

	refreshBtn := widget.NewButton("Refresh", func() {
		pollCircuits()
	})

	autoRefreshCheck := widget.NewCheck("Auto-refresh", func(on bool) {
		autoRefresh = on
		if on {
			tickerDone = make(chan struct{})
			ticker = time.NewTicker(5 * time.Second)
			go func() {
				for {
					select {
					case <-ticker.C:
						pollCircuits()
					case <-tickerDone:
						return
					}
				}
			}()
			pollCircuits() // immediate first poll
		} else {
			if ticker != nil {
				ticker.Stop()
			}
			if tickerDone != nil {
				close(tickerDone)
			}
		}
	})

	// Close Circuit button.
	closeBtn := widget.NewButton("Close Circuit", func() {
		mu.Lock()
		idx := selectedIdx
		var circID string
		if idx >= 0 && idx < len(circuits) {
			circID = circuits[idx].info.ID
		}
		mu.Unlock()

		if circID == "" {
			dialog.ShowInformation("Close Circuit", "No circuit selected.", a.window)
			return
		}
		if a.engine.TorControl == nil {
			dialog.ShowError(fmt.Errorf("Tor control not connected"), a.window)
			return
		}

		if err := a.engine.TorControl.CloseCircuit(circID); err != nil {
			dialog.ShowError(fmt.Errorf("close circuit #%s: %w", circID, err), a.window)
			return
		}
		a.logger.Info("closed circuit #%s", circID)
		pollCircuits()
	})

	// Block Relay button.
	blockBtn := widget.NewButton("Block Relay", func() {
		mu.Lock()
		idx := selectedIdx
		var relays []tor.RelayInfo
		if idx >= 0 && idx < len(circuits) {
			relays = circuits[idx].resolved
		}
		mu.Unlock()

		if len(relays) == 0 {
			dialog.ShowInformation("Block Relay", "No circuit selected.", a.window)
			return
		}

		// Show a selection dialog with relay entries.
		var options []string
		for _, ri := range relays {
			label := ri.Fingerprint
			if ri.Nickname != "" {
				label = ri.Nickname + " (" + ri.Fingerprint + ")"
			}
			if ri.CountryCode != "" {
				label += " [" + ri.CountryCode + "]"
			}
			options = append(options, label)
		}

		relaySelect := widget.NewSelect(options, nil)
		relaySelect.PlaceHolder = "Select relay to block..."

		dlgContent := container.NewVBox(
			widget.NewLabel("Select a relay to add to ExcludeNodes:"),
			relaySelect,
		)

		d := dialog.NewCustomConfirm("Block Relay", "Block", "Cancel", dlgContent, func(ok bool) {
			if !ok || relaySelect.SelectedIndex() < 0 {
				return
			}
			ri := relays[relaySelect.SelectedIndex()]
			fp := ri.Fingerprint
			if !strings.HasPrefix(fp, "$") {
				fp = "$" + fp
			}

			if !containsEntry(a.cfg.Relays.ExcludeNodes, fp) {
				a.cfg.Relays.ExcludeNodes = append(a.cfg.Relays.ExcludeNodes, fp)
				a.logger.Info("blocked relay %s (%s)", fp, ri.Nickname)

				// Hot-reload if running.
				if a.engine.State() == lifecycle.StateRunning {
					if err := a.engine.ReloadConfig(a.cfg); err != nil {
						a.logger.Error("reload config after blocking relay: %v", err)
					}
				}
			}
			dialog.ShowInformation("Relay Blocked", fmt.Sprintf("Added %s to ExcludeNodes", fp), a.window)
		}, a.window)
		d.Show()
	})

	// Globe node tap handler: block the tapped relay.
	globe.OnNodeTapped = func(fingerprint string) {
		fp := fingerprint
		if !strings.HasPrefix(fp, "$") {
			fp = "$" + fp
		}
		dialog.ShowConfirm("Block Relay",
			fmt.Sprintf("Add %s to ExcludeNodes?", fp),
			func(ok bool) {
				if !ok {
					return
				}
				if !containsEntry(a.cfg.Relays.ExcludeNodes, fp) {
					a.cfg.Relays.ExcludeNodes = append(a.cfg.Relays.ExcludeNodes, fp)
					a.logger.Info("blocked relay %s via globe", fp)
					if a.engine.State() == lifecycle.StateRunning {
						if err := a.engine.ReloadConfig(a.cfg); err != nil {
							a.logger.Error("reload config after blocking relay: %v", err)
						}
					}
				}
			}, a.window)
	}

	// Listen for state changes to auto-start/stop polling.
	a.engine.OnStateChange(func(_, to lifecycle.State) {
		if to == lifecycle.StateRunning && autoRefresh {
			pollCircuits()
		}
	})

	toolbar := container.NewHBox(refreshBtn, autoRefreshCheck, countLabel)
	actionBar := container.NewHBox(closeBtn, blockBtn)

	leftPanel := container.NewBorder(nil, actionBar, nil, nil, circuitList)

	split := container.NewHSplit(leftPanel, globe)
	split.Offset = 0.35

	return container.NewBorder(toolbar, nil, nil, nil, split)
}

// relayRole returns the role string for a relay at position idx in a path of length n.
func relayRole(idx, n int) string {
	if n == 0 {
		return "middle"
	}
	if idx == 0 {
		return "guard"
	}
	if idx == n-1 {
		return "exit"
	}
	return "middle"
}

// formatCircuitPath returns a human-readable path summary for a circuit.
func formatCircuitPath(ce circuitEntry) string {
	var parts []string
	for _, ri := range ce.resolved {
		name := ri.Nickname
		if name == "" {
			name = ri.Fingerprint
			if len(name) > 10 {
				name = name[:10] + "..."
			}
		}
		if ri.CountryCode != "" {
			name += "[" + ri.CountryCode + "]"
		}
		parts = append(parts, name)
	}
	if len(parts) == 0 {
		return strings.Join(ce.info.Path, " → ")
	}
	return strings.Join(parts, " → ")
}
