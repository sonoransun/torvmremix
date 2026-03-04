package config

import "testing"

func TestDiffNoChanges(t *testing.T) {
	a := DefaultConfig()
	b := DefaultConfig()
	diff := Diff(a, b)
	if diff.HasChanges() {
		t.Fatal("expected no changes between identical configs")
	}
}

func TestDiffHotReloadableBridge(t *testing.T) {
	a := DefaultConfig()
	b := DefaultConfig()
	b.Bridge.UseBridges = true
	b.Bridge.Transport = "obfs4"

	diff := Diff(a, b)
	if !diff.HasChanges() {
		t.Fatal("expected changes")
	}
	if len(diff.HotReloadable) != 1 {
		t.Fatalf("expected 1 hot-reloadable change, got %d: %v", len(diff.HotReloadable), diff.HotReloadable)
	}
	if len(diff.RestartRequired) != 0 {
		t.Fatalf("expected 0 restart-required changes, got %d: %v", len(diff.RestartRequired), diff.RestartRequired)
	}
}

func TestDiffHotReloadableProxy(t *testing.T) {
	a := DefaultConfig()
	b := DefaultConfig()
	b.Proxy.Type = "socks5"
	b.Proxy.Address = "127.0.0.1:1080"

	diff := Diff(a, b)
	if !diff.HasChanges() {
		t.Fatal("expected changes")
	}
	found := false
	for _, h := range diff.HotReloadable {
		if h == "Proxy (proxy)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Proxy in hot-reloadable, got %v", diff.HotReloadable)
	}
}

func TestDiffHotReloadableVerbose(t *testing.T) {
	a := DefaultConfig()
	b := DefaultConfig()
	b.Verbose = true

	diff := Diff(a, b)
	if !diff.HasChanges() {
		t.Fatal("expected changes")
	}
	found := false
	for _, h := range diff.HotReloadable {
		if h == "Verbose (verbose)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Verbose in hot-reloadable, got %v", diff.HotReloadable)
	}
}

func TestDiffRestartRequired(t *testing.T) {
	a := DefaultConfig()
	b := DefaultConfig()
	b.VMMemoryMB = 256
	b.VMCPUs = 4

	diff := Diff(a, b)
	if !diff.HasChanges() {
		t.Fatal("expected changes")
	}
	if len(diff.RestartRequired) != 2 {
		t.Fatalf("expected 2 restart-required changes, got %d: %v", len(diff.RestartRequired), diff.RestartRequired)
	}
	if len(diff.HotReloadable) != 0 {
		t.Fatalf("expected 0 hot-reloadable changes, got %d", len(diff.HotReloadable))
	}
}

func TestDiffMixed(t *testing.T) {
	a := DefaultConfig()
	b := DefaultConfig()
	b.Verbose = true     // hot-reloadable
	b.VMMemoryMB = 512   // restart-required

	diff := Diff(a, b)
	if len(diff.HotReloadable) != 1 {
		t.Fatalf("expected 1 hot-reloadable, got %d", len(diff.HotReloadable))
	}
	if len(diff.RestartRequired) != 1 {
		t.Fatalf("expected 1 restart-required, got %d", len(diff.RestartRequired))
	}
}
