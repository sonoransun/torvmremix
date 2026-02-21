package vm

import (
	"fmt"
	"os"
	"os/exec"
)

// WriteStateDiskFile writes content to a file inside an ext4 disk image
// using debugfs. This avoids needing root or mount privileges.
func WriteStateDiskFile(diskPath, guestPath, content string) error {
	// Write content to a temporary file.
	tmp, err := os.CreateTemp("", "torvm-overlay-*")
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
