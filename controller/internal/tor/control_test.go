package tor

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// mockTorServer creates a TCP listener that simulates a Tor Control port.
// It returns the listener address and a channel that receives accepted connections.
func mockTorServer(t *testing.T) (string, chan net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	conns := make(chan net.Conn, 1)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conns <- conn
		}
	}()
	return ln.Addr().String(), conns
}

// readCommand reads a single command line from the mock connection.
func readCommand(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func TestAuthenticateSuccess(t *testing.T) {
	addr, conns := mockTorServer(t)

	go func() {
		conn := <-conns
		defer conn.Close()
		r := bufio.NewReader(conn)

		cmd, _ := readCommand(r)
		if !strings.HasPrefix(cmd, "AUTHENTICATE") {
			t.Errorf("expected AUTHENTICATE, got %q", cmd)
		}
		fmt.Fprintf(conn, "250 OK\r\n")
	}()

	client, err := NewControlClient(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	if err := client.Authenticate("testpass"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
}

func TestAuthenticateFailure(t *testing.T) {
	addr, conns := mockTorServer(t)

	go func() {
		conn := <-conns
		defer conn.Close()
		r := bufio.NewReader(conn)

		readCommand(r)
		fmt.Fprintf(conn, "515 Bad authentication\r\n")
	}()

	client, err := NewControlClient(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	err = client.Authenticate("wrong")
	if err == nil {
		t.Fatal("expected authentication error")
	}
	if !strings.Contains(err.Error(), "515") {
		t.Fatalf("expected 515 error, got: %v", err)
	}
}

func TestGetInfoSingleKeyword(t *testing.T) {
	addr, conns := mockTorServer(t)

	done := make(chan struct{})
	go func() {
		conn := <-conns
		defer conn.Close()
		r := bufio.NewReader(conn)

		cmd, _ := readCommand(r)
		if !strings.HasPrefix(cmd, "GETINFO version") {
			t.Errorf("expected GETINFO version, got %q", cmd)
		}
		fmt.Fprintf(conn, "250-version=0.4.8.12\r\n250 OK\r\n")
		<-done
	}()

	client, err := NewControlClient(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { close(done); client.Close() }()

	result, err := client.GetInfo("version")
	if err != nil {
		t.Fatalf("getinfo: %v", err)
	}
	if result["version"] != "0.4.8.12" {
		t.Fatalf("expected version 0.4.8.12, got %q", result["version"])
	}
}

func TestGetInfoMultiKeyword(t *testing.T) {
	addr, conns := mockTorServer(t)

	done := make(chan struct{})
	go func() {
		conn := <-conns
		defer conn.Close()
		r := bufio.NewReader(conn)

		cmd, _ := readCommand(r)
		if !strings.Contains(cmd, "GETINFO") {
			t.Errorf("expected GETINFO, got %q", cmd)
		}
		fmt.Fprintf(conn, "250-version=0.4.8.12\r\n250-config-file=/etc/tor/torrc\r\n250 OK\r\n")
		<-done
	}()

	client, err := NewControlClient(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { close(done); client.Close() }()

	result, err := client.GetInfo("version", "config-file")
	if err != nil {
		t.Fatalf("getinfo: %v", err)
	}
	if result["version"] != "0.4.8.12" {
		t.Fatalf("expected version 0.4.8.12, got %q", result["version"])
	}
	if result["config-file"] != "/etc/tor/torrc" {
		t.Fatalf("expected config-file /etc/tor/torrc, got %q", result["config-file"])
	}
}

func TestSignalSuccess(t *testing.T) {
	addr, conns := mockTorServer(t)

	go func() {
		conn := <-conns
		defer conn.Close()
		r := bufio.NewReader(conn)

		cmd, _ := readCommand(r)
		if cmd != "SIGNAL NEWNYM" {
			t.Errorf("expected SIGNAL NEWNYM, got %q", cmd)
		}
		fmt.Fprintf(conn, "250 OK\r\n")
	}()

	client, err := NewControlClient(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	if err := client.Signal("NEWNYM"); err != nil {
		t.Fatalf("signal: %v", err)
	}
}

func TestSetConfSuccess(t *testing.T) {
	addr, conns := mockTorServer(t)

	go func() {
		conn := <-conns
		defer conn.Close()
		r := bufio.NewReader(conn)

		cmd, _ := readCommand(r)
		if !strings.HasPrefix(cmd, "SETCONF") {
			t.Errorf("expected SETCONF, got %q", cmd)
		}
		fmt.Fprintf(conn, "250 OK\r\n")
	}()

	client, err := NewControlClient(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	err = client.SetConf(map[string]string{"MaxCircuitDirtiness": "600"})
	if err != nil {
		t.Fatalf("setconf: %v", err)
	}
}

func TestSetConfFailure(t *testing.T) {
	addr, conns := mockTorServer(t)

	go func() {
		conn := <-conns
		defer conn.Close()
		r := bufio.NewReader(conn)

		readCommand(r)
		fmt.Fprintf(conn, "552 Unrecognized option\r\n")
	}()

	client, err := NewControlClient(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	err = client.SetConf(map[string]string{"BadOption": "value"})
	if err == nil {
		t.Fatal("expected setconf error")
	}
	if !strings.Contains(err.Error(), "552") {
		t.Fatalf("expected 552 error, got: %v", err)
	}
}

func TestAsyncEventDelivery(t *testing.T) {
	addr, conns := mockTorServer(t)

	go func() {
		conn := <-conns
		defer conn.Close()
		r := bufio.NewReader(conn)

		// Handle SETEVENTS.
		readCommand(r)
		fmt.Fprintf(conn, "250 OK\r\n")

		// Send async BW event.
		time.Sleep(50 * time.Millisecond)
		fmt.Fprintf(conn, "650 BW 1024 2048\r\n")
	}()

	client, err := NewControlClient(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	if err := client.SetEvents([]string{"BW"}); err != nil {
		t.Fatalf("setevents: %v", err)
	}

	select {
	case ev := <-client.Events():
		if ev.Code != 650 {
			t.Fatalf("expected code 650, got %d", ev.Code)
		}
		if ev.Action != "BW" {
			t.Fatalf("expected action BW, got %q", ev.Action)
		}
		if len(ev.Lines) == 0 || !strings.Contains(ev.Lines[0], "1024") {
			t.Fatalf("unexpected event lines: %v", ev.Lines)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async event")
	}
}

func TestProtocolInjectionPrevention(t *testing.T) {
	addr, conns := mockTorServer(t)

	go func() {
		conn := <-conns
		defer conn.Close()
		// Just hold the connection open.
		<-make(chan struct{})
	}()

	client, err := NewControlClient(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	// Test newline in authenticate password.
	if err := client.Authenticate("pass\nSIGNAL SHUTDOWN"); err == nil {
		t.Fatal("expected error for newline in password")
	}

	// Test carriage return in signal.
	if err := client.Signal("NEWNYM\rSIGNAL SHUTDOWN"); err == nil {
		t.Fatal("expected error for CR in signal")
	}

	// Test newline in GetInfo keyword.
	if _, err := client.GetInfo("version\nSIGNAL SHUTDOWN"); err == nil {
		t.Fatal("expected error for newline in keyword")
	}

	// Test newline in SetConf key.
	if err := client.SetConf(map[string]string{"key\n": "val"}); err == nil {
		t.Fatal("expected error for newline in setconf key")
	}

	// Test newline in SetConf value.
	if err := client.SetConf(map[string]string{"key": "val\n"}); err == nil {
		t.Fatal("expected error for newline in setconf value")
	}

	// Test newline in SetEvents.
	if err := client.SetEvents([]string{"BW\nSIGNAL SHUTDOWN"}); err == nil {
		t.Fatal("expected error for newline in event name")
	}
}

func TestConnectionClose(t *testing.T) {
	addr, conns := mockTorServer(t)

	go func() {
		conn := <-conns
		defer conn.Close()
		// Hold connection open until client closes.
		buf := make([]byte, 1)
		conn.Read(buf)
	}()

	client, err := NewControlClient(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Closing again should not error.
	if err := client.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestParseBootstrapStatus(t *testing.T) {
	line := `NOTICE BOOTSTRAP PROGRESS=50 TAG=loading_descriptors SUMMARY="Loading relay descriptors"`
	status, err := ParseBootstrapStatus(line)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if status.Progress != 50 {
		t.Fatalf("expected progress 50, got %d", status.Progress)
	}
	if status.Tag != "loading_descriptors" {
		t.Fatalf("expected tag loading_descriptors, got %q", status.Tag)
	}
	if status.Summary != "Loading relay descriptors" {
		t.Fatalf("expected summary 'Loading relay descriptors', got %q", status.Summary)
	}
}

func TestParseBootstrapStatusComplete(t *testing.T) {
	line := `NOTICE BOOTSTRAP PROGRESS=100 TAG=done SUMMARY="Done"`
	status, err := ParseBootstrapStatus(line)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if status.Progress != 100 {
		t.Fatalf("expected progress 100, got %d", status.Progress)
	}
}

func TestParseBandwidthEvent(t *testing.T) {
	stats, err := ParseBandwidthEvent("BW 1024 2048")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stats.BytesRead != 1024 {
		t.Fatalf("expected 1024 read, got %d", stats.BytesRead)
	}
	if stats.BytesWritten != 2048 {
		t.Fatalf("expected 2048 written, got %d", stats.BytesWritten)
	}
}

func TestParseBandwidthEventWith650Prefix(t *testing.T) {
	stats, err := ParseBandwidthEvent("650 BW 500 300")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stats.BytesRead != 500 || stats.BytesWritten != 300 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestParseCircuitLine(t *testing.T) {
	ci := parseCircuitLine("5 BUILT $relay1,$relay2,$relay3 PURPOSE=GENERAL")
	if ci.ID != "5" {
		t.Fatalf("expected ID 5, got %q", ci.ID)
	}
	if ci.Status != "BUILT" {
		t.Fatalf("expected status BUILT, got %q", ci.Status)
	}
	if len(ci.Path) != 3 {
		t.Fatalf("expected 3 path hops, got %d", len(ci.Path))
	}
	if ci.Purpose != "GENERAL" {
		t.Fatalf("expected purpose GENERAL, got %q", ci.Purpose)
	}
}

func TestParseCircuitLineNoPath(t *testing.T) {
	ci := parseCircuitLine("1 LAUNCHED PURPOSE=GENERAL")
	if ci.ID != "1" {
		t.Fatalf("expected ID 1, got %q", ci.ID)
	}
	if ci.Status != "LAUNCHED" {
		t.Fatalf("expected status LAUNCHED, got %q", ci.Status)
	}
	if ci.Purpose != "GENERAL" {
		t.Fatalf("expected purpose GENERAL, got %q", ci.Purpose)
	}
}
