package config

import "encoding/json"

// ConfigVersion is the current configuration schema version.
const ConfigVersion = 1

// configEnvelope is used to peek at the version field before full unmarshal.
type configEnvelope struct {
	Version int `json:"config_version"`
}

// migrateJSON applies any necessary migrations to raw config JSON bytes,
// returning the migrated JSON. If the version is already current, the
// input is returned unchanged.
func migrateJSON(data []byte) ([]byte, error) {
	var env configEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		// If we can't parse the version, treat as v0 (pre-versioning).
		env.Version = 0
	}

	// Apply migration chain.
	for env.Version < ConfigVersion {
		switch env.Version {
		case 0:
			// v0 → v1: Add config_version field. No structural changes
			// needed — the DefaultConfig merge in Load() handles new fields.
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				return data, nil // can't migrate, use as-is
			}
			vb, _ := json.Marshal(1)
			raw["config_version"] = vb
			migrated, err := json.MarshalIndent(raw, "", "  ")
			if err != nil {
				return data, nil
			}
			data = migrated
			env.Version = 1
		default:
			// Unknown version; return as-is and let validation catch issues.
			return data, nil
		}
	}

	return data, nil
}
