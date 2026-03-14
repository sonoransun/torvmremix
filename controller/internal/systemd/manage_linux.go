//go:build linux

package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	unitName = "torvm.service"
	unitPath = "/etc/systemd/system/" + unitName
	// bundledUnit is the path to the unit file shipped with the binary.
	bundledUnit = "/usr/local/share/torvm/torvm.service"
)

// ServiceStatus holds the current state of the systemd service.
type ServiceStatus struct {
	Installed bool
	Active    bool
	Enabled   bool
	PID       string
	Status    string
}

// Install copies the unit file and enables the service.
func Install() error {
	// Read bundled unit file.
	data, err := os.ReadFile(bundledUnit)
	if err != nil {
		return fmt.Errorf("systemd: read unit file %s: %w", bundledUnit, err)
	}

	if err := os.WriteFile(unitPath, data, 0644); err != nil {
		return fmt.Errorf("systemd: write unit file: %w", err)
	}

	if err := systemctl("daemon-reload"); err != nil {
		return fmt.Errorf("systemd: daemon-reload: %w", err)
	}

	if err := systemctl("enable", unitName); err != nil {
		return fmt.Errorf("systemd: enable: %w", err)
	}

	return nil
}

// Uninstall stops, disables, and removes the service unit file.
func Uninstall() error {
	_ = systemctl("stop", unitName)
	_ = systemctl("disable", unitName)
	os.Remove(unitPath)
	_ = systemctl("daemon-reload")
	return nil
}

// Start starts the systemd service.
func Start() error {
	return systemctl("start", unitName)
}

// Stop stops the systemd service.
func Stop() error {
	return systemctl("stop", unitName)
}

// Restart restarts the systemd service.
func Restart() error {
	return systemctl("restart", unitName)
}

// Enable enables the service to start on boot.
func Enable() error {
	return systemctl("enable", unitName)
}

// Disable disables the service from starting on boot.
func Disable() error {
	return systemctl("disable", unitName)
}

// QueryStatus returns the current service status.
func QueryStatus() ServiceStatus {
	st := ServiceStatus{}

	// Check if unit file exists.
	if _, err := os.Stat(unitPath); err != nil {
		return st
	}
	st.Installed = true

	// Check if active.
	if err := systemctl("is-active", "--quiet", unitName); err == nil {
		st.Active = true
	}

	// Check if enabled.
	if err := systemctl("is-enabled", "--quiet", unitName); err == nil {
		st.Enabled = true
	}

	// Get PID and status from show.
	out, err := systemctlOutput("show", unitName,
		"--property=MainPID,StatusText", "--no-pager")
	if err == nil {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "MainPID=") {
				pid := strings.TrimPrefix(line, "MainPID=")
				if pid != "0" {
					st.PID = pid
				}
			}
			if strings.HasPrefix(line, "StatusText=") {
				st.Status = strings.TrimPrefix(line, "StatusText=")
			}
		}
	}

	return st
}

// ReadJournalLogs returns the last n lines of journal output for the service.
func ReadJournalLogs(n int) (string, error) {
	out, err := systemctlOutput("--no-pager", "-n", fmt.Sprintf("%d", n),
		"-u", unitName)
	if err != nil {
		// Try journalctl directly.
		cmd := exec.Command("journalctl", "--no-pager", "-n",
			fmt.Sprintf("%d", n), "-u", unitName)
		b, jErr := cmd.CombinedOutput()
		if jErr != nil {
			return "", fmt.Errorf("systemd: read journal: %w", jErr)
		}
		return string(b), nil
	}
	return out, nil
}

func systemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func systemctlOutput(args ...string) (string, error) {
	cmd := exec.Command("systemctl", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
