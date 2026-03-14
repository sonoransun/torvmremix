package gui

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/user/extorvm/controller/internal/lifecycle"
	"github.com/user/extorvm/controller/internal/tor"
)

var (
	guiFingerprintRe = regexp.MustCompile(`^\$[0-9a-fA-F]{40}$`)
	guiCountryCodeRe = regexp.MustCompile(`^\{[a-zA-Z]{2}\}$`)
	guiBareHexRe     = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
)

// countryName maps ISO 3166-1 alpha-2 codes to display names for the selector.
var countryName = map[string]string{
	"US": "United States", "GB": "United Kingdom", "DE": "Germany",
	"FR": "France", "NL": "Netherlands", "RU": "Russia",
	"CN": "China", "IN": "India", "CA": "Canada",
	"AU": "Australia", "JP": "Japan", "BR": "Brazil",
	"SE": "Sweden", "CH": "Switzerland", "IT": "Italy",
	"ES": "Spain", "UA": "Ukraine", "PL": "Poland",
	"RO": "Romania", "CZ": "Czech Republic", "AT": "Austria",
	"FI": "Finland", "NO": "Norway", "DK": "Denmark",
	"SG": "Singapore", "HK": "Hong Kong", "KR": "South Korea",
	"TW": "Taiwan", "ZA": "South Africa", "IL": "Israel",
	"TR": "Turkey", "IR": "Iran",
}

// relaysTab builds the Relays tab for excluding relays by fingerprint or country.
func (a *App) relaysTab() fyne.CanvasObject {
	header := widget.NewLabel("Relay Exclusion")
	header.TextStyle = fyne.TextStyle{Bold: true}

	// --- ExcludeNodes section ---
	excludeLabel := widget.NewLabel("Exclude Nodes (all circuit positions)")
	excludeLabel.TextStyle = fyne.TextStyle{Bold: true}

	var excludeMu sync.Mutex
	var excludeList *widget.List
	excludeList = widget.NewList(
		func() int {
			excludeMu.Lock()
			defer excludeMu.Unlock()
			return len(a.cfg.Relays.ExcludeNodes)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("placeholder entry text"),
				layout.NewSpacer(),
				widget.NewButton("Remove", nil),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			excludeMu.Lock()
			entry := ""
			if id < len(a.cfg.Relays.ExcludeNodes) {
				entry = a.cfg.Relays.ExcludeNodes[id]
			}
			excludeMu.Unlock()

			box := obj.(*fyne.Container)
			label := box.Objects[0].(*widget.Label)
			btn := box.Objects[2].(*widget.Button)
			label.SetText(formatExcludeEntry(entry))
			idx := id
			btn.OnTapped = func() {
				excludeMu.Lock()
				if idx < len(a.cfg.Relays.ExcludeNodes) {
					a.cfg.Relays.ExcludeNodes = append(
						a.cfg.Relays.ExcludeNodes[:idx],
						a.cfg.Relays.ExcludeNodes[idx+1:]...,
					)
				}
				excludeMu.Unlock()
				excludeList.Refresh()
			}
		},
	)

	excludeEntry := widget.NewEntry()
	excludeEntry.SetPlaceHolder("$fingerprint or {CC}")
	excludeAddBtn := widget.NewButton("Add", func() {
		entry := normalizeRelayEntry(excludeEntry.Text)
		if err := validateGUIRelayEntry(entry); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		excludeMu.Lock()
		if !containsEntry(a.cfg.Relays.ExcludeNodes, entry) {
			a.cfg.Relays.ExcludeNodes = append(a.cfg.Relays.ExcludeNodes, entry)
		}
		excludeMu.Unlock()
		excludeEntry.SetText("")
		excludeList.Refresh()
	})
	excludeRow := container.NewBorder(nil, nil, nil, excludeAddBtn, excludeEntry)

	// --- ExcludeExitNodes section ---
	exitLabel := widget.NewLabel("Exclude Exit Nodes (exit position only)")
	exitLabel.TextStyle = fyne.TextStyle{Bold: true}

	var exitMu sync.Mutex
	var exitList *widget.List
	exitList = widget.NewList(
		func() int {
			exitMu.Lock()
			defer exitMu.Unlock()
			return len(a.cfg.Relays.ExcludeExitNodes)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("placeholder entry text"),
				layout.NewSpacer(),
				widget.NewButton("Remove", nil),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			exitMu.Lock()
			entry := ""
			if id < len(a.cfg.Relays.ExcludeExitNodes) {
				entry = a.cfg.Relays.ExcludeExitNodes[id]
			}
			exitMu.Unlock()

			box := obj.(*fyne.Container)
			label := box.Objects[0].(*widget.Label)
			btn := box.Objects[2].(*widget.Button)
			label.SetText(formatExcludeEntry(entry))
			idx := id
			btn.OnTapped = func() {
				exitMu.Lock()
				if idx < len(a.cfg.Relays.ExcludeExitNodes) {
					a.cfg.Relays.ExcludeExitNodes = append(
						a.cfg.Relays.ExcludeExitNodes[:idx],
						a.cfg.Relays.ExcludeExitNodes[idx+1:]...,
					)
				}
				exitMu.Unlock()
				exitList.Refresh()
			}
		},
	)

	exitEntry := widget.NewEntry()
	exitEntry.SetPlaceHolder("$fingerprint or {CC}")
	exitAddBtn := widget.NewButton("Add", func() {
		entry := normalizeRelayEntry(exitEntry.Text)
		if err := validateGUIRelayEntry(entry); err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		exitMu.Lock()
		if !containsEntry(a.cfg.Relays.ExcludeExitNodes, entry) {
			a.cfg.Relays.ExcludeExitNodes = append(a.cfg.Relays.ExcludeExitNodes, entry)
		}
		exitMu.Unlock()
		exitEntry.SetText("")
		exitList.Refresh()
	})
	exitRow := container.NewBorder(nil, nil, nil, exitAddBtn, exitEntry)

	// --- Country quick-add with human-readable names ---
	countryCodes := []string{
		"US", "GB", "DE", "FR", "NL", "RU", "CN", "IN",
		"CA", "AU", "JP", "BR", "SE", "CH", "IT", "ES",
		"UA", "PL", "RO", "CZ", "AT", "FI", "NO", "DK",
		"SG", "HK", "KR", "TW", "ZA", "IL", "TR", "IR",
	}
	var countryOptions []string
	for _, cc := range countryCodes {
		name := countryName[cc]
		countryOptions = append(countryOptions, fmt.Sprintf("{%s} %s", cc, name))
	}

	countrySelect := widget.NewSelect(countryOptions, nil)
	countrySelect.PlaceHolder = "Select country..."

	extractCC := func() string {
		sel := countrySelect.Selected
		if len(sel) < 4 {
			return ""
		}
		// Format is "{XX} Name" — extract the {XX} part.
		return sel[:4]
	}

	addToExclude := widget.NewButton("Exclude All", func() {
		cc := extractCC()
		if cc == "" {
			return
		}
		excludeMu.Lock()
		if !containsEntry(a.cfg.Relays.ExcludeNodes, cc) {
			a.cfg.Relays.ExcludeNodes = append(a.cfg.Relays.ExcludeNodes, cc)
		}
		excludeMu.Unlock()
		excludeList.Refresh()
	})
	addToExitExclude := widget.NewButton("Exclude Exit", func() {
		cc := extractCC()
		if cc == "" {
			return
		}
		exitMu.Lock()
		if !containsEntry(a.cfg.Relays.ExcludeExitNodes, cc) {
			a.cfg.Relays.ExcludeExitNodes = append(a.cfg.Relays.ExcludeExitNodes, cc)
		}
		exitMu.Unlock()
		exitList.Refresh()
	})

	countryRow := container.NewHBox(
		widget.NewLabel("Country:"),
		countrySelect,
		addToExclude,
		addToExitExclude,
	)

	// --- Block from Active Circuits ---
	activeLabel := widget.NewLabel("Block from Active Circuits")
	activeLabel.TextStyle = fyne.TextStyle{Bold: true}

	var activeRelaysMu sync.Mutex
	var activeRelays []tor.RelayInfo

	activeList := widget.NewList(
		func() int {
			activeRelaysMu.Lock()
			defer activeRelaysMu.Unlock()
			return len(activeRelays)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("placeholder relay text here"),
				layout.NewSpacer(),
				widget.NewButton("Block", nil),
				widget.NewButton("Block Exit", nil),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			activeRelaysMu.Lock()
			var ri tor.RelayInfo
			if id < len(activeRelays) {
				ri = activeRelays[id]
			}
			activeRelaysMu.Unlock()

			box := obj.(*fyne.Container)
			label := box.Objects[0].(*widget.Label)
			blockBtn := box.Objects[2].(*widget.Button)
			blockExitBtn := box.Objects[3].(*widget.Button)

			display := ri.Fingerprint
			if ri.Nickname != "" {
				display = ri.Nickname + " " + ri.Fingerprint
			}
			if ri.CountryCode != "" {
				display += " [" + ri.CountryCode + "]"
			}
			if ri.Role != "" {
				display += " (" + ri.Role + ")"
			}
			label.SetText(display)

			fp := ri.Fingerprint
			if fp != "" && !strings.HasPrefix(fp, "$") {
				fp = "$" + fp
			}

			blockBtn.OnTapped = func() {
				if fp == "" {
					return
				}
				excludeMu.Lock()
				if !containsEntry(a.cfg.Relays.ExcludeNodes, fp) {
					a.cfg.Relays.ExcludeNodes = append(a.cfg.Relays.ExcludeNodes, fp)
					a.logger.Info("blocked relay %s (ExcludeNodes)", fp)
				}
				excludeMu.Unlock()
				excludeList.Refresh()
				a.hotReloadRelays()
			}
			blockExitBtn.OnTapped = func() {
				if fp == "" {
					return
				}
				exitMu.Lock()
				if !containsEntry(a.cfg.Relays.ExcludeExitNodes, fp) {
					a.cfg.Relays.ExcludeExitNodes = append(a.cfg.Relays.ExcludeExitNodes, fp)
					a.logger.Info("blocked relay %s (ExcludeExitNodes)", fp)
				}
				exitMu.Unlock()
				exitList.Refresh()
				a.hotReloadRelays()
			}
		},
	)

	fetchRelaysBtn := widget.NewButton("Fetch Active Relays", func() {
		if a.engine.TorControl == nil || a.engine.State() != lifecycle.StateRunning {
			dialog.ShowInformation("Not Connected", "TorVM must be running to fetch active relays.", a.window)
			return
		}
		circs, err := a.engine.TorControl.GetCircuits()
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}

		seen := make(map[string]bool)
		var relays []tor.RelayInfo
		for _, ci := range circs {
			if ci.Status != "BUILT" && ci.Status != "EXTENDED" {
				continue
			}
			for i, entry := range ci.Path {
				fp, nick := tor.ParseRelayPath(entry)
				if seen[fp] {
					continue
				}
				seen[fp] = true

				ri := tor.RelayInfo{
					Fingerprint: fp,
					Nickname:    nick,
					Role:        relayRole(i, len(ci.Path)),
				}
				// Resolve country if control port is available.
				if resolved, err := a.engine.TorControl.ResolveRelay(entry); err == nil {
					ri.CountryCode = resolved.CountryCode
					ri.Latitude = resolved.Latitude
					ri.Longitude = resolved.Longitude
					ri.Role = relayRole(i, len(ci.Path))
				}
				relays = append(relays, ri)
			}
		}

		activeRelaysMu.Lock()
		activeRelays = relays
		activeRelaysMu.Unlock()
		activeList.Refresh()
	})

	// --- StrictNodes ---
	strictCheck := widget.NewCheck("Strict Nodes (never use excluded relays, even if it breaks circuits)", func(on bool) {
		a.cfg.Relays.StrictNodes = on
	})
	strictCheck.Checked = a.cfg.Relays.StrictNodes

	// Use fixed-height containers for the lists.
	excludeListBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(600, 120)), excludeList)
	exitListBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(600, 120)), exitList)
	activeListBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(600, 130)), activeList)

	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		excludeLabel,
		excludeRow,
		excludeListBox,
		widget.NewSeparator(),
		exitLabel,
		exitRow,
		exitListBox,
		widget.NewSeparator(),
		countryRow,
		widget.NewSeparator(),
		strictCheck,
		widget.NewSeparator(),
		activeLabel,
		fetchRelaysBtn,
		activeListBox,
		layout.NewSpacer(),
	)

	return container.NewScroll(content)
}

// hotReloadRelays applies relay exclusion changes to the running Tor instance.
func (a *App) hotReloadRelays() {
	if a.engine.State() == lifecycle.StateRunning {
		if err := a.engine.ReloadConfig(a.cfg); err != nil {
			a.logger.Error("relay hot-reload: %v", err)
		}
	}
}

// formatExcludeEntry returns a human-readable label for an exclusion entry.
func formatExcludeEntry(entry string) string {
	if len(entry) == 4 && entry[0] == '{' && entry[3] == '}' {
		cc := strings.ToUpper(entry[1:3])
		if name, ok := countryName[cc]; ok {
			return entry + " " + name
		}
	}
	return entry
}

// normalizeRelayEntry trims whitespace, auto-prepends $ for bare hex
// fingerprints, and uppercases country codes.
func normalizeRelayEntry(s string) string {
	s = strings.TrimSpace(s)
	if guiBareHexRe.MatchString(s) {
		return "$" + s
	}
	if len(s) == 4 && s[0] == '{' && s[3] == '}' {
		return strings.ToUpper(s)
	}
	return s
}

// validateGUIRelayEntry checks that an entry is a valid fingerprint or country code.
func validateGUIRelayEntry(entry string) error {
	if guiFingerprintRe.MatchString(entry) || guiCountryCodeRe.MatchString(entry) {
		return nil
	}
	return fmt.Errorf("invalid entry %q: must be a fingerprint ($hex40) or country code ({XX})", entry)
}

// containsEntry checks if a slice contains a given string.
func containsEntry(slice []string, entry string) bool {
	for _, s := range slice {
		if s == entry {
			return true
		}
	}
	return false
}
