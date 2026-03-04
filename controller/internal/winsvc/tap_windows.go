//go:build windows

package winsvc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TAPStatus describes the current state of the TAP-Windows adapter.
type TAPStatus struct {
	Installed   bool
	AdapterName string
	DriverPath  string
}

// QueryTAPStatus checks whether a TAP-Windows6 adapter is installed by
// querying the network adapters via netsh.
func QueryTAPStatus() *TAPStatus {
	st := &TAPStatus{}

	out, err := exec.Command("netsh", "interface", "show", "interface").Output()
	if err != nil {
		return st
	}

	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "tap") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				st.Installed = true
				st.AdapterName = fields[len(fields)-1]
			}
			break
		}
	}

	// Check for the TAP driver in common install locations.
	programFiles := os.Getenv("ProgramFiles")
	if programFiles == "" {
		programFiles = `C:\Program Files`
	}
	tapDir := filepath.Join(programFiles, "TAP-Windows", "bin")
	if _, err := os.Stat(tapDir); err == nil {
		st.DriverPath = tapDir
	}

	return st
}

// InstallTAPAdapter runs the TAP-Windows6 addtap.bat script to add a
// new TAP adapter. The TAP-Windows driver must already be installed on
// the system (typically bundled with the TorVM installer).
func InstallTAPAdapter() error {
	batPath := findTAPBat("addtap.bat")
	if batPath == "" {
		return fmt.Errorf("winsvc: addtap.bat not found; install TAP-Windows6 driver first")
	}

	cmd := exec.Command(batPath)
	cmd.Dir = filepath.Dir(batPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("winsvc: addtap failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveTAPAdapter runs the TAP-Windows6 deltapall.bat script to remove
// all TAP adapters. Use with caution as this removes ALL TAP adapters.
func RemoveTAPAdapter() error {
	batPath := findTAPBat("deltapall.bat")
	if batPath == "" {
		return fmt.Errorf("winsvc: deltapall.bat not found; TAP-Windows6 driver may not be installed")
	}

	cmd := exec.Command(batPath)
	cmd.Dir = filepath.Dir(batPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("winsvc: deltapall failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// findTAPBat searches common TAP-Windows installation paths for the
// specified batch file.
func findTAPBat(name string) string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "TAP-Windows", "bin", name),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "TAP-Windows", "bin", name),
		filepath.Join(`C:\Program Files`, "TAP-Windows", "bin", name),
		filepath.Join(`C:\Program Files (x86)`, "TAP-Windows", "bin", name),
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
