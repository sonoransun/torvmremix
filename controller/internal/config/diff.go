package config

import (
	"fmt"
	"reflect"
)

// ConfigDiff categorizes the differences between two Config values.
type ConfigDiff struct {
	// HotReloadable lists field names that changed and can be applied
	// at runtime via the Tor Control Protocol (bridges, proxy, verbose).
	HotReloadable []string

	// RestartRequired lists field names that changed but require a
	// full VM restart to take effect (memory, CPUs, paths, ports, etc.).
	RestartRequired []string
}

// HasChanges returns true if any fields differ between old and new.
func (d ConfigDiff) HasChanges() bool {
	return len(d.HotReloadable) > 0 || len(d.RestartRequired) > 0
}

// hotReloadableFields lists Config fields that can be applied at runtime.
var hotReloadableFields = map[string]bool{
	"Bridge":  true,
	"Proxy":   true,
	"Verbose": true,
}

// Diff compares old and new Config and returns a ConfigDiff describing what
// changed. Fields are categorised as hot-reloadable or restart-required.
func Diff(old, new *Config) ConfigDiff {
	var diff ConfigDiff

	oldV := reflect.ValueOf(*old)
	newV := reflect.ValueOf(*new)
	t := oldV.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported and runtime-only fields.
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}

		oldField := oldV.Field(i)
		newField := newV.Field(i)

		if reflect.DeepEqual(oldField.Interface(), newField.Interface()) {
			continue
		}

		name := field.Name
		label := fmt.Sprintf("%s (%s)", name, tag)

		if hotReloadableFields[name] {
			diff.HotReloadable = append(diff.HotReloadable, label)
		} else {
			diff.RestartRequired = append(diff.RestartRequired, label)
		}
	}

	return diff
}
