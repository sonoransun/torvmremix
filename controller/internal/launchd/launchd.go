//go:build darwin

package launchd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	serviceLabel = "org.torproject.torvm"
	plistPath    = "/Library/LaunchDaemons/org.torproject.torvm.plist"
	logDir       = "/var/log/torvm"
	logPath      = "/var/log/torvm/torvm.log"
	binaryPath   = "/usr/local/bin/torvm"
)

// Status describes the current state of the launchd service.
type Status struct {
	Installed  bool
	Running    bool
	RunAtLoad  bool
	PID        int
	LastExit   int
	PlistPath  string
}

// QueryStatus checks the plist file and launchctl output to determine service state.
func QueryStatus() *Status {
	st := &Status{PlistPath: plistPath}

	// Check if plist exists on disk.
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return st
	}
	st.Installed = true
	st.RunAtLoad = strings.Contains(string(data), "<key>RunAtLoad</key>\n\t<true/>")

	// Query launchctl for running state.
	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return st
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, serviceLabel) {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				if fields[0] != "-" {
					fmt.Sscanf(fields[0], "%d", &st.PID)
					st.Running = st.PID > 0
				}
				fmt.Sscanf(fields[1], "%d", &st.LastExit)
			}
			break
		}
	}

	return st
}

// Install generates the plist and writes it to /Library/LaunchDaemons/ via privilege escalation.
func Install(runAtLoad bool) error {
	plist := generatePlist(runAtLoad)

	// Use a temporary file to stage the plist content.
	tmp, err := os.CreateTemp("", "torvm-plist-*.plist")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(plist); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp plist: %w", err)
	}
	tmp.Close()

	cmd := fmt.Sprintf(
		"mkdir -p '%s' && cp '%s' '%s' && chmod 644 '%s' && launchctl load '%s'",
		logDir, tmpPath, plistPath, plistPath, plistPath,
	)
	return runPrivileged(cmd)
}

// Uninstall unloads and removes the plist via privilege escalation.
func Uninstall() error {
	cmd := fmt.Sprintf(
		"launchctl unload '%s' 2>/dev/null; rm -f '%s'",
		plistPath, plistPath,
	)
	return runPrivileged(cmd)
}

// Start kicks the service via launchctl.
func Start() error {
	return runPrivileged(fmt.Sprintf(
		"launchctl kickstart -k system/%s",
		serviceLabel,
	))
}

// Stop sends SIGTERM to the service via launchctl.
func Stop() error {
	return runPrivileged(fmt.Sprintf(
		"launchctl kill SIGTERM system/%s",
		serviceLabel,
	))
}

// SetRunAtLoad modifies the RunAtLoad key in the installed plist.
func SetRunAtLoad(enabled bool) error {
	cmd := fmt.Sprintf(
		`/usr/libexec/PlistBuddy -c "Set :RunAtLoad %t" '%s'`,
		enabled, plistPath,
	)
	return runPrivileged(cmd)
}

// ReadLog returns the last n lines of the service log.
func ReadLog(lines int) (string, error) {
	out, err := exec.Command("tail", "-n", fmt.Sprintf("%d", lines), logPath).Output()
	if err != nil {
		return "", fmt.Errorf("read log: %w", err)
	}
	return string(out), nil
}

// runPrivileged executes a shell command with admin privileges via osascript.
func runPrivileged(command string) error {
	script := fmt.Sprintf(
		`do shell script "%s" with administrator privileges`,
		strings.ReplaceAll(command, `"`, `\"`),
	)
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("privileged command failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// generatePlist creates the launchd plist XML.
func generatePlist(runAtLoad bool) string {
	runAtLoadStr := "false"
	if runAtLoad {
		runAtLoadStr = "true"
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--headless</string>
	</array>
	<key>RunAtLoad</key>
	<%s/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, serviceLabel, binaryPath, runAtLoadStr, logPath, logPath)
}
