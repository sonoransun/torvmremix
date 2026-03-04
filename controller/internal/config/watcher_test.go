package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestConfigWatcherDetectsChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write initial config.
	cfg := DefaultConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var received *Config

	watcher, err := NewConfigWatcher(path, func(c *Config) {
		mu.Lock()
		defer mu.Unlock()
		received = c
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	// Modify config file.
	cfg.Verbose = true
	data, err = json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce + processing.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := received
		mu.Unlock()
		if got != nil {
			if !got.Verbose {
				t.Error("expected Verbose=true in reloaded config")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for config change callback")
}

func TestConfigWatcherCloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	data, _ := json.Marshal(cfg)
	os.WriteFile(path, data, 0600)

	watcher, err := NewConfigWatcher(path, func(c *Config) {})
	if err != nil {
		t.Fatal(err)
	}

	if err := watcher.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := watcher.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestConfigWatcherInvalidConfigIgnored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := DefaultConfig()
	data, _ := json.Marshal(cfg)
	os.WriteFile(path, data, 0600)

	callCount := 0
	var mu sync.Mutex

	watcher, err := NewConfigWatcher(path, func(c *Config) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	// Write invalid JSON.
	os.WriteFile(path, []byte("{invalid json}"), 0600)

	time.Sleep(600 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 0 {
		t.Fatalf("expected 0 callbacks for invalid config, got %d", count)
	}
}
