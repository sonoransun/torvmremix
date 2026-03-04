package lifecycle

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/network"
	"github.com/user/extorvm/controller/internal/testutil"
)

// mockVM implements VMController for testing.
type mockVM struct {
	mu         sync.Mutex
	running    bool
	startErr   error
	stopErr    error
	waitCh     chan error // closed/sent when VM "exits"
	startCount int
	stopCount  int
}

func newMockVM() *mockVM {
	return &mockVM{waitCh: make(chan error, 1)}
}

func (m *mockVM) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCount++
	if m.startErr != nil {
		return m.startErr
	}
	m.running = true
	return nil
}

func (m *mockVM) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCount++
	m.running = false
	if m.stopErr != nil {
		return m.stopErr
	}
	return nil
}

func (m *mockVM) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *mockVM) Wait(ctx context.Context) error {
	select {
	case err := <-m.waitCh:
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SimulateExit makes the mock VM exit with the given error.
func (m *mockVM) SimulateExit(err error) {
	m.waitCh <- err
}

// mockNetwork implements network.Manager for testing.
type mockNetwork struct {
	mu               sync.Mutex
	createTAPErr     error
	destroyTAPErr    error
	saveConfigErr    error
	restoreConfigErr error
	setupRoutingErr  error
	teardownErr      error
	flushDNSErr      error

	createTAPCount     int
	destroyTAPCount    int
	saveConfigCount    int
	restoreConfigCount int
	setupRoutingCount  int
	teardownCount      int
	flushDNSCount      int
}

func (m *mockNetwork) CreateTAP(name string, hostIP, vmIP net.IP, mask net.IPMask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createTAPCount++
	return m.createTAPErr
}

func (m *mockNetwork) DestroyTAP(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.destroyTAPCount++
	return m.destroyTAPErr
}

func (m *mockNetwork) SaveConfig() (*network.SavedConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveConfigCount++
	if m.saveConfigErr != nil {
		return nil, m.saveConfigErr
	}
	return &network.SavedConfig{Data: []byte("mock"), Platform: "test"}, nil
}

func (m *mockNetwork) RestoreConfig(cfg *network.SavedConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restoreConfigCount++
	return m.restoreConfigErr
}

func (m *mockNetwork) SetupRouting(tapName string, vmIP net.IP) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setupRoutingCount++
	return m.setupRoutingErr
}

func (m *mockNetwork) TeardownRouting() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.teardownCount++
	return m.teardownErr
}

func (m *mockNetwork) FlushDNS() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushDNSCount++
	return m.flushDNSErr
}

// testConfig returns a minimal valid config for lifecycle tests.
func testConfig() *config.Config {
	return &config.Config{
		HostIP:     "10.10.10.2",
		VMIP:       "10.10.10.1",
		SubnetMask: "255.255.255.252",
		TAPName:    "tap0",
		SOCKSPort:  9050,
		ControlPort: 9051,
	}
}

// newTestEngine creates a lifecycle engine with mock dependencies and no
// retry policies (for deterministic tests).
func newTestEngine() (*Engine, *mockVM, *mockNetwork) {
	logger, _ := testutil.NewTestLogger()
	cfg := testConfig()
	vm := newMockVM()
	net := &mockNetwork{}

	e := NewEngineWithDeps(cfg, logger, vm, net)
	// Disable all retries for deterministic tests.
	e.retryPolicy = map[State]*RetryPolicy{}
	return e, vm, net
}

func TestNewEngineWithDeps(t *testing.T) {
	e, vm, net := newTestEngine()
	if e.VM != vm {
		t.Error("VM not set correctly")
	}
	if e.Network != net {
		t.Error("Network not set correctly")
	}
	if e.State() != StateInit {
		t.Errorf("initial state = %v, want StateInit", e.State())
	}
	if e.FailSafe == nil {
		t.Error("FailSafe not initialized")
	}
}

func TestTransitionCallsObservers(t *testing.T) {
	e, _, _ := newTestEngine()

	var transitions []struct{ from, to State }
	e.OnStateChange(func(from, to State) {
		transitions = append(transitions, struct{ from, to State }{from, to})
	})

	e.transition(StateCheckPrivileges)
	e.transition(StateSaveNetwork)

	if len(transitions) != 2 {
		t.Fatalf("got %d transitions, want 2", len(transitions))
	}
	if transitions[0].from != StateInit || transitions[0].to != StateCheckPrivileges {
		t.Errorf("transition[0] = %v->%v, want Init->CheckPrivileges", transitions[0].from, transitions[0].to)
	}
	if transitions[1].from != StateCheckPrivileges || transitions[1].to != StateSaveNetwork {
		t.Errorf("transition[1] = %v->%v, want CheckPrivileges->SaveNetwork", transitions[1].from, transitions[1].to)
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateInit, "Init"},
		{StateRunning, "Running"},
		{StateFailed, "Failed"},
		{State(99), "State(99)"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestDoSaveNetwork(t *testing.T) {
	e, _, _ := newTestEngine()
	e.state = StateSaveNetwork

	if err := e.doSaveNetwork(); err != nil {
		t.Fatal(err)
	}
	if e.state != StateCreateTAP {
		t.Errorf("state = %v, want StateCreateTAP", e.state)
	}
	if e.savedNet == nil {
		t.Error("savedNet not set")
	}
}

func TestDoSaveNetworkError(t *testing.T) {
	e, _, net := newTestEngine()
	e.state = StateSaveNetwork
	net.saveConfigErr = fmt.Errorf("mock save error")

	if err := e.doSaveNetwork(); err == nil {
		t.Error("expected error, got nil")
	}
	if e.state != StateSaveNetwork {
		t.Errorf("state should not change on error, got %v", e.state)
	}
}

func TestDoCreateTAP(t *testing.T) {
	e, _, _ := newTestEngine()
	e.state = StateCreateTAP

	if err := e.doCreateTAP(); err != nil {
		t.Fatal(err)
	}
	if e.state != StateLaunchVM {
		t.Errorf("state = %v, want StateLaunchVM", e.state)
	}
}

func TestDoCreateTAPError(t *testing.T) {
	e, _, net := newTestEngine()
	e.state = StateCreateTAP
	net.createTAPErr = fmt.Errorf("mock TAP error")

	if err := e.doCreateTAP(); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDoLaunchVM(t *testing.T) {
	e, _, _ := newTestEngine()
	e.state = StateLaunchVM

	ctx := context.Background()
	if err := e.doLaunchVM(ctx); err != nil {
		t.Fatal(err)
	}
	if e.state != StateWaitTAP {
		t.Errorf("state = %v, want StateWaitTAP", e.state)
	}
}

func TestDoLaunchVMError(t *testing.T) {
	e, vm, _ := newTestEngine()
	e.state = StateLaunchVM
	vm.startErr = fmt.Errorf("mock start error")

	ctx := context.Background()
	if err := e.doLaunchVM(ctx); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDoRunningVMExit(t *testing.T) {
	e, vm, _ := newTestEngine()
	e.state = StateRunning

	ctx := context.Background()
	// Simulate VM exiting normally after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		vm.SimulateExit(nil)
	}()

	if err := e.doRunning(ctx); err != nil {
		t.Fatal(err)
	}
	if e.state != StateShutdown {
		t.Errorf("state = %v, want StateShutdown", e.state)
	}
	if e.FailSafe.IsActive() {
		t.Error("failsafe should not be active on normal exit")
	}
}

func TestDoRunningVMUnexpectedExit(t *testing.T) {
	e, vm, _ := newTestEngine()
	e.state = StateRunning

	ctx := context.Background()
	go func() {
		time.Sleep(50 * time.Millisecond)
		vm.SimulateExit(fmt.Errorf("crash"))
	}()

	if err := e.doRunning(ctx); err != nil {
		t.Fatal(err)
	}
	if !e.FailSafe.IsActive() {
		t.Error("failsafe should be active on unexpected VM exit")
	}
}

func TestDoRunningContextCancel(t *testing.T) {
	e, _, _ := newTestEngine()
	e.state = StateRunning

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	if err := e.doRunning(ctx); err != nil {
		t.Fatal(err)
	}
	if e.state != StateShutdown {
		t.Errorf("state = %v, want StateShutdown", e.state)
	}
}

func TestDoShutdown(t *testing.T) {
	e, vm, _ := newTestEngine()
	e.state = StateShutdown
	vm.running = true

	ctx := context.Background()
	if err := e.doShutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if e.state != StateRestoreNetwork {
		t.Errorf("state = %v, want StateRestoreNetwork", e.state)
	}

	vm.mu.Lock()
	stopped := vm.stopCount
	vm.mu.Unlock()
	if stopped < 1 {
		t.Error("Stop should have been called")
	}
}

func TestDoRestoreNetwork(t *testing.T) {
	e, _, net := newTestEngine()
	e.state = StateRestoreNetwork
	e.savedNet = &network.SavedConfig{Data: []byte("saved"), Platform: "test"}

	if err := e.doRestoreNetwork(); err != nil {
		t.Fatal(err)
	}
	if e.state != StateCleanup {
		t.Errorf("state = %v, want StateCleanup", e.state)
	}

	net.mu.Lock()
	teardowns := net.teardownCount
	restores := net.restoreConfigCount
	destroys := net.destroyTAPCount
	net.mu.Unlock()
	if teardowns < 1 {
		t.Error("TeardownRouting should have been called")
	}
	if restores < 1 {
		t.Error("RestoreConfig should have been called")
	}
	if destroys < 1 {
		t.Error("DestroyTAP should have been called")
	}
}

func TestDoCleanup(t *testing.T) {
	e, _, _ := newTestEngine()
	e.FailSafe.active = true

	err := e.doCleanup()
	if err != nil {
		t.Fatal(err)
	}
	if e.FailSafe.IsActive() {
		t.Error("failsafe should be deactivated after cleanup")
	}
}

func TestDoFlushDNS(t *testing.T) {
	e, _, _ := newTestEngine()
	e.state = StateFlushDNS

	if err := e.doFlushDNS(); err != nil {
		t.Fatal(err)
	}
	if e.state != StateWaitBootstrap {
		t.Errorf("state = %v, want StateWaitBootstrap", e.state)
	}
}

func TestDoFlushDNSErrorNonFatal(t *testing.T) {
	e, _, net := newTestEngine()
	e.state = StateFlushDNS
	net.flushDNSErr = fmt.Errorf("mock flush error")

	// FlushDNS logs the error but transitions anyway.
	if err := e.doFlushDNS(); err != nil {
		t.Fatal(err)
	}
	if e.state != StateWaitBootstrap {
		t.Errorf("state = %v, want StateWaitBootstrap", e.state)
	}
}

func TestDoConfigureTAPError(t *testing.T) {
	e, _, net := newTestEngine()
	e.state = StateConfigureTAP
	net.setupRoutingErr = fmt.Errorf("mock routing error")

	if err := e.doConfigureTAP(); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestDoRestoreNetworkTeardownFailure(t *testing.T) {
	e, _, net := newTestEngine()
	e.state = StateRestoreNetwork
	net.teardownErr = fmt.Errorf("teardown failed")

	if err := e.doRestoreNetwork(); err != nil {
		t.Fatal(err)
	}
	// Failsafe should activate when teardown fails.
	if !e.FailSafe.IsActive() {
		t.Error("failsafe should be active when teardown fails")
	}
}

func TestReloadConfigNoChanges(t *testing.T) {
	e, _, _ := newTestEngine()
	e.state = StateRunning

	newCfg := testConfig()
	if err := e.ReloadConfig(newCfg); err != nil {
		t.Fatal(err)
	}
}

func TestReloadConfigHotReloadableWithoutTorControl(t *testing.T) {
	e, _, _ := newTestEngine()
	e.state = StateRunning

	newCfg := testConfig()
	newCfg.Verbose = true
	if err := e.ReloadConfig(newCfg); err != nil {
		t.Fatal(err)
	}
	// Config should be updated even without TorControl.
	if !e.Config.Verbose {
		t.Error("expected Config.Verbose to be true after reload")
	}
}

func TestReloadConfigRestartRequired(t *testing.T) {
	e, _, _ := newTestEngine()
	e.state = StateRunning

	newCfg := testConfig()
	newCfg.VMMemoryMB = 256
	if err := e.ReloadConfig(newCfg); err != nil {
		t.Fatal(err)
	}
	// Config pointer should be updated.
	if e.Config.VMMemoryMB != 256 {
		t.Error("expected Config.VMMemoryMB to be 256 after reload")
	}
}

func TestParseTorrcOverlay(t *testing.T) {
	overlay := "UseBridges 1\nClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy\n"
	directives := parseTorrcOverlay(overlay)
	if directives["UseBridges"] != "1" {
		t.Errorf("UseBridges = %q, want %q", directives["UseBridges"], "1")
	}
	if directives["ClientTransportPlugin"] != "obfs4 exec /usr/bin/obfs4proxy" {
		t.Errorf("ClientTransportPlugin = %q", directives["ClientTransportPlugin"])
	}
}
