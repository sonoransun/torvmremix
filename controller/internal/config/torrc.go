package config

import (
	"fmt"
	"strings"
)

// sanitizeTorrcLine rejects values containing newline characters that could
// inject additional torrc directives.
func sanitizeTorrcLine(field, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("torrc %s contains newline characters", field)
	}
	return nil
}

// TorrcOverlay generates torrc configuration lines from Bridge and Proxy settings.
// Returns an empty string and nil error if no overlay is needed.
func (c *Config) TorrcOverlay() (string, error) {
	var lines []string

	// Bridge / pluggable transport configuration.
	if c.Bridge.UseBridges {
		lines = append(lines, "UseBridges 1")

		switch c.Bridge.Transport {
		case "obfs4":
			lines = append(lines, "ClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy")
		case "meek-azure":
			lines = append(lines, "ClientTransportPlugin meek_lite exec /usr/bin/obfs4proxy")
		case "snowflake":
			lines = append(lines, "ClientTransportPlugin snowflake exec /usr/bin/snowflake-client")
		case "", "none":
			// no transport plugin needed
		default:
			return "", fmt.Errorf("unsupported bridge transport: %q", c.Bridge.Transport)
		}

		for _, b := range c.Bridge.Bridges {
			b = strings.TrimSpace(b)
			if b != "" {
				if err := sanitizeTorrcLine("bridge", b); err != nil {
					return "", err
				}
				lines = append(lines, fmt.Sprintf("Bridge %s", b))
			}
		}
	}

	// Upstream proxy configuration.
	if c.Proxy.Type != "" && c.Proxy.Address != "" {
		if err := sanitizeTorrcLine("proxy address", c.Proxy.Address); err != nil {
			return "", err
		}
		if err := sanitizeTorrcLine("proxy username", c.Proxy.Username); err != nil {
			return "", err
		}
		if err := sanitizeTorrcLine("proxy password", c.Proxy.Password); err != nil {
			return "", err
		}

		switch strings.ToLower(c.Proxy.Type) {
		case "http":
			lines = append(lines, fmt.Sprintf("HTTPProxy %s", c.Proxy.Address))
			if c.Proxy.Username != "" {
				lines = append(lines, fmt.Sprintf("HTTPProxyAuthenticator %s:%s", c.Proxy.Username, c.Proxy.Password))
			}
		case "https":
			lines = append(lines, fmt.Sprintf("HTTPSProxy %s", c.Proxy.Address))
			if c.Proxy.Username != "" {
				lines = append(lines, fmt.Sprintf("HTTPSProxyAuthenticator %s:%s", c.Proxy.Username, c.Proxy.Password))
			}
		case "socks5":
			lines = append(lines, fmt.Sprintf("Socks5Proxy %s", c.Proxy.Address))
			if c.Proxy.Username != "" {
				lines = append(lines, fmt.Sprintf("Socks5ProxyUsername %s", c.Proxy.Username))
				lines = append(lines, fmt.Sprintf("Socks5ProxyPassword %s", c.Proxy.Password))
			}
		}
	}

	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}
