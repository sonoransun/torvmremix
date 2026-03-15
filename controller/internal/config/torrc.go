package config

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// bridgeLineRe validates a bridge line format. A bridge line typically contains
// a transport name, IP:port, and a hex fingerprint with optional parameters.
// This rejects control characters and most shell-special characters.
var bridgeLineRe = regexp.MustCompile(`^[a-zA-Z0-9.:[\] /=,+_-]+$`)

// fingerprintRe matches a Tor relay fingerprint: $ followed by 40 hex characters.
var fingerprintRe = regexp.MustCompile(`^\$[0-9a-fA-F]{40}$`)

// countryCodeRe matches a Tor country code selector: {XX} where XX is two letters.
var countryCodeRe = regexp.MustCompile(`^\{[a-zA-Z]{2}\}$`)

// credentialRe validates proxy username/password characters. Only allows
// printable ASCII excluding characters that could break torrc parsing.
var credentialRe = regexp.MustCompile(`^[a-zA-Z0-9!@#$%^&*()_+=[\]{}<>,.?/~-]+$`)

const maxCredentialLen = 255

// sanitizeTorrcLine rejects values containing newline characters or other
// control characters that could inject additional torrc directives.
func sanitizeTorrcLine(field, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("torrc %s contains newline characters", field)
	}
	// Reject any other control characters (ASCII < 32 or DEL).
	for _, c := range value {
		if c < 32 || c == 127 {
			return fmt.Errorf("torrc %s contains control character 0x%02x", field, c)
		}
	}
	return nil
}

// validateBridgeLine validates a bridge configuration line format.
func validateBridgeLine(line string) error {
	if err := sanitizeTorrcLine("bridge", line); err != nil {
		return err
	}
	if len(line) > 1024 {
		return fmt.Errorf("bridge line too long (%d chars, max 1024)", len(line))
	}
	if !bridgeLineRe.MatchString(line) {
		return fmt.Errorf("bridge line contains invalid characters: %q", line)
	}
	return nil
}

// validateProxyAddress validates a proxy address is a valid host:port.
func validateProxyAddress(addr string) error {
	if err := sanitizeTorrcLine("proxy address", addr); err != nil {
		return err
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("proxy address %q is not valid host:port: %w", addr, err)
	}
	if host == "" {
		return fmt.Errorf("proxy address has empty host")
	}
	if port == "" {
		return fmt.Errorf("proxy address has empty port")
	}
	return nil
}

// validateCredential validates a proxy username or password.
func validateCredential(field, value string) error {
	if value == "" {
		return nil
	}
	if err := sanitizeTorrcLine(field, value); err != nil {
		return err
	}
	if len(value) > maxCredentialLen {
		return fmt.Errorf("%s too long (%d chars, max %d)", field, len(value), maxCredentialLen)
	}
	if !credentialRe.MatchString(value) {
		return fmt.Errorf("%s contains invalid characters", field)
	}
	return nil
}

// validateRelayEntry validates a single relay exclusion entry.
// Accepts either a fingerprint ($hex40) or country code ({XX}).
func validateRelayEntry(entry string) error {
	if err := sanitizeTorrcLine("relay entry", entry); err != nil {
		return err
	}
	if !fingerprintRe.MatchString(entry) && !countryCodeRe.MatchString(entry) {
		return fmt.Errorf("relay entry %q must be a fingerprint ($hex40) or country code ({XX})", entry)
	}
	return nil
}

// validateRelayConfig validates all entries in a RelayConfig.
func validateRelayConfig(rc *RelayConfig) error {
	for _, e := range rc.ExcludeNodes {
		if err := validateRelayEntry(e); err != nil {
			return fmt.Errorf("ExcludeNodes: %w", err)
		}
	}
	for _, e := range rc.ExcludeExitNodes {
		if err := validateRelayEntry(e); err != nil {
			return fmt.Errorf("ExcludeExitNodes: %w", err)
		}
	}
	return nil
}

// TorrcOverlay generates torrc configuration lines from Bridge, Proxy, and Relay settings.
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
				if err := validateBridgeLine(b); err != nil {
					return "", err
				}
				lines = append(lines, fmt.Sprintf("Bridge %s", b))
			}
		}
	}

	// Upstream proxy configuration.
	if c.Proxy.Type != "" && c.Proxy.Address != "" {
		if err := validateProxyAddress(c.Proxy.Address); err != nil {
			return "", err
		}
		if err := validateCredential("proxy username", c.Proxy.Username); err != nil {
			return "", err
		}
		if err := validateCredential("proxy password", c.Proxy.Password); err != nil {
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

	// Relay exclusion configuration.
	if err := validateRelayConfig(&c.Relays); err != nil {
		return "", err
	}
	if len(c.Relays.ExcludeNodes) > 0 {
		lines = append(lines, fmt.Sprintf("ExcludeNodes %s", strings.Join(c.Relays.ExcludeNodes, ",")))
	}
	if len(c.Relays.ExcludeExitNodes) > 0 {
		lines = append(lines, fmt.Sprintf("ExcludeExitNodes %s", strings.Join(c.Relays.ExcludeExitNodes, ",")))
	}
	if c.Relays.StrictNodes {
		lines = append(lines, "StrictNodes 1")
	}

	// FHE hidden service configuration for encrypted index sharing.
	if c.FHE.Enabled && c.FHE.ShareEnabled && c.FHE.HiddenServicePort > 0 {
		lines = append(lines, "HiddenServiceDir /home/tor/hidden_service/fhe_index")
		lines = append(lines, fmt.Sprintf("HiddenServicePort %d %s:%d",
			c.FHE.HiddenServicePort, c.HostIP, c.FHE.HiddenServicePort))
	}

	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}
