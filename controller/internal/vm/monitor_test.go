package vm

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// mockQMPServer simulates a QMP server on a Unix socket.
type mockQMPServer struct {
	listener net.Listener
	sockPath string
}

func newMockQMPServer(t *testing.T) *mockQMPServer {
	t.Helper()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "qmp.sock")

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen on %s: %v", sockPath, err)
	}
	return &mockQMPServer{listener: l, sockPath: sockPath}
}

func (s *mockQMPServer) Close() {
	s.listener.Close()
	os.Remove(s.sockPath)
}

// serve handles a single client connection with the standard QMP handshake.
// handler is called for each command after qmp_capabilities.
func (s *mockQMPServer) serve(handler func(cmd string, enc *json.Encoder)) {
	go func() {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		enc := json.NewEncoder(conn)
		dec := json.NewDecoder(conn)

		// Send QMP greeting.
		greeting := map[string]interface{}{
			"QMP": map[string]interface{}{
				"version": map[string]interface{}{
					"qemu": map[string]int{"major": 8, "minor": 0, "micro": 0},
				},
				"capabilities": []string{},
			},
		}
		enc.Encode(greeting)

		// Read qmp_capabilities.
		var cmd qmpCommand
		if err := dec.Decode(&cmd); err != nil {
			return
		}
		// Reply with success.
		enc.Encode(map[string]interface{}{"return": map[string]interface{}{}})

		// Handle subsequent commands.
		for {
			var cmd qmpCommand
			if err := dec.Decode(&cmd); err != nil {
				return
			}
			if handler != nil {
				handler(cmd.Execute, enc)
			}
		}
	}()
}

func TestNewQMPClientGreeting(t *testing.T) {
	srv := newMockQMPServer(t)
	defer srv.Close()
	srv.serve(nil)

	client, err := NewQMPClient(srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
}

func TestNewQMPClientBadSocket(t *testing.T) {
	_, err := NewQMPClient("/nonexistent/qmp.sock")
	if err == nil {
		t.Error("expected error connecting to nonexistent socket")
	}
}

func TestSystemPowerdown(t *testing.T) {
	srv := newMockQMPServer(t)
	defer srv.Close()

	receivedCmd := make(chan string, 1)
	srv.serve(func(cmd string, enc *json.Encoder) {
		receivedCmd <- cmd
		enc.Encode(map[string]interface{}{"return": map[string]interface{}{}})
	})

	client, err := NewQMPClient(srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.SystemPowerdown(); err != nil {
		t.Fatal(err)
	}

	cmd := <-receivedCmd
	if cmd != "system_powerdown" {
		t.Errorf("got command %q, want system_powerdown", cmd)
	}
}

func TestQueryStatus(t *testing.T) {
	srv := newMockQMPServer(t)
	defer srv.Close()

	srv.serve(func(cmd string, enc *json.Encoder) {
		if cmd == "query-status" {
			resp := map[string]interface{}{
				"return": map[string]interface{}{
					"status":  "running",
					"running": true,
				},
			}
			enc.Encode(resp)
		}
	})

	client, err := NewQMPClient(srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	status, running, err := client.QueryStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
	if !running {
		t.Error("expected running = true")
	}
}

func TestQueryStatusPaused(t *testing.T) {
	srv := newMockQMPServer(t)
	defer srv.Close()

	srv.serve(func(cmd string, enc *json.Encoder) {
		if cmd == "query-status" {
			resp := map[string]interface{}{
				"return": map[string]interface{}{
					"status":  "paused",
					"running": false,
				},
			}
			enc.Encode(resp)
		}
	})

	client, err := NewQMPClient(srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	status, running, err := client.QueryStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status != "paused" {
		t.Errorf("status = %q, want paused", status)
	}
	if running {
		t.Error("expected running = false")
	}
}

func TestQMPErrorResponse(t *testing.T) {
	srv := newMockQMPServer(t)
	defer srv.Close()

	srv.serve(func(cmd string, enc *json.Encoder) {
		resp := map[string]interface{}{
			"error": map[string]string{
				"class": "GenericError",
				"desc":  "test error",
			},
		}
		enc.Encode(resp)
	})

	client, err := NewQMPClient(srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	err = client.SystemPowerdown()
	if err == nil {
		t.Error("expected error from QMP error response")
	}
	if err != nil {
		errMsg := fmt.Sprintf("%v", err)
		if !contains(errMsg, "GenericError") || !contains(errMsg, "test error") {
			t.Errorf("error should contain class and desc, got: %v", err)
		}
	}
}

func TestQMPClientClose(t *testing.T) {
	srv := newMockQMPServer(t)
	defer srv.Close()
	srv.serve(nil)

	client, err := NewQMPClient(srv.sockPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	// After close, commands should fail.
	err = client.SystemPowerdown()
	if err == nil {
		t.Error("expected error after close")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
