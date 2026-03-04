//go:build linux

package systemd

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

// IsRunningUnderSystemd returns true if the process was started by systemd,
// indicated by the presence of the NOTIFY_SOCKET environment variable.
func IsRunningUnderSystemd() bool {
	return os.Getenv("NOTIFY_SOCKET") != ""
}

// Ready sends READY=1 to systemd, signalling that the service has finished
// starting up and is ready to serve.
func Ready() error {
	return notify("READY=1")
}

// Stopping sends STOPPING=1 to systemd, signalling that the service is
// beginning its shutdown sequence.
func Stopping() error {
	return notify("STOPPING=1")
}

// Watchdog sends WATCHDOG=1 to systemd, resetting the watchdog timer.
// This must be called at intervals less than WatchdogSec/2.
func Watchdog() error {
	return notify("WATCHDOG=1")
}

// Status sends a free-form status string to systemd, displayed by
// "systemctl status".
func Status(status string) error {
	return notify("STATUS=" + status)
}

// MainPID sends MAINPID=<pid> to systemd, identifying the main process.
func MainPID(pid int) error {
	return notify("MAINPID=" + strconv.Itoa(pid))
}

// notify sends a sd_notify datagram to the socket specified by the
// NOTIFY_SOCKET environment variable. The protocol uses Unix datagram
// sockets. Abstract sockets are indicated by a leading '@' which is
// replaced with a null byte.
func notify(state string) error {
	socketAddr := os.Getenv("NOTIFY_SOCKET")
	if socketAddr == "" {
		return fmt.Errorf("systemd: NOTIFY_SOCKET not set")
	}

	// Abstract sockets start with '@', replace with null byte.
	if socketAddr[0] == '@' {
		socketAddr = "\x00" + socketAddr[1:]
	}

	conn, err := net.DialUnix("unixgram", nil, &net.UnixAddr{
		Name: socketAddr,
		Net:  "unixgram",
	})
	if err != nil {
		return fmt.Errorf("systemd: dial notify socket: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(state))
	if err != nil {
		return fmt.Errorf("systemd: write notify socket: %w", err)
	}

	return nil
}
