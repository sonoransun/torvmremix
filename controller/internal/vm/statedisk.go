package vm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var guestPathRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

// validateGuestPath ensures a guest filesystem path is safe for use with debugfs.
func validateGuestPath(p string) error {
	if p == "" {
		return fmt.Errorf("guest path must not be empty")
	}
	if len(p) > 255 {
		return fmt.Errorf("guest path too long (%d chars, max 255)", len(p))
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("guest path must not contain '..'")
	}
	if !guestPathRe.MatchString(p) {
		return fmt.Errorf("guest path contains invalid characters: %q", p)
	}
	return nil
}

// WriteStateDiskFile writes content to a file inside an ext4 disk image
// using debugfs. This avoids needing root or mount privileges.
func WriteStateDiskFile(diskPath, guestPath, content string) error {
	// Validate guest path to prevent injection into debugfs commands.
	if err := validateGuestPath(guestPath); err != nil {
		return fmt.Errorf("invalid guest path: %w", err)
	}

	// Validate disk path is an existing regular file.
	fi, err := os.Stat(diskPath)
	if err != nil {
		return fmt.Errorf("stat disk image: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("disk path is not a regular file: %s", diskPath)
	}

	// Use temp dir co-located with disk path for safety.
	tmpDir := filepath.Dir(diskPath)
	if _, err := os.Stat(tmpDir); err != nil {
		tmpDir = os.TempDir()
	}

	// Write content to a temporary file.
	tmp, err := os.CreateTemp(tmpDir, "torvm-overlay-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	// Use debugfs to write the temp file into the disk image.
	cmd := exec.Command("debugfs", "-w", "-R",
		fmt.Sprintf("write %s %s", tmp.Name(), guestPath),
		diskPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("debugfs write: %w: %s", err, string(out))
	}
	return nil
}
