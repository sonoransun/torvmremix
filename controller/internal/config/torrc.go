package config

import (
	"fmt"
	"strings"
)

// TorrcOverlay generates torrc configuration lines from Bridge and Proxy settings.
// Returns an empty string if no overlay is needed.
func (c *Config) TorrcOverlay() string {
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
		}

		for _, b := range c.Bridge.Bridges {
			b = strings.TrimSpace(b)
			if b != "" {
				lines = append(lines, fmt.Sprintf("Bridge %s", b))
			}
		}
	}

	// Upstream proxy configuration.
	if c.Proxy.Type != "" && c.Proxy.Address != "" {
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
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
