package tor

import (
	"fmt"
	"strings"
)

// RelayInfo holds resolved information about a Tor relay.
type RelayInfo struct {
	Fingerprint string
	Nickname    string
	CountryCode string
	Latitude    float64
	Longitude   float64
	Role        string // "guard", "middle", "exit" — set by caller
}

// ParseRelayPath parses a circuit path entry like "$FINGERPRINT~Nickname"
// into its fingerprint and nickname components.
func ParseRelayPath(entry string) (fingerprint, nickname string) {
	entry = strings.TrimSpace(entry)
	if idx := strings.IndexByte(entry, '~'); idx >= 0 {
		return entry[:idx], entry[idx+1:]
	}
	return entry, ""
}

// GetRelayCountry resolves an IP address to a two-letter country code
// using Tor's built-in GeoIP database.
func (c *ControlClient) GetRelayCountry(ip string) (string, error) {
	if err := validateNoNewlines(ip); err != nil {
		return "", fmt.Errorf("tor: relay country: %w", err)
	}
	info, err := c.GetInfo("ip-to-country/" + ip)
	if err != nil {
		return "", err
	}
	cc, ok := info["ip-to-country/"+ip]
	if !ok || cc == "" {
		return "", fmt.Errorf("tor: no country for IP %s", ip)
	}
	return strings.ToUpper(strings.TrimSpace(cc)), nil
}

// GetRelayAddress resolves a relay fingerprint to its IP address
// using Tor's network status data. The fingerprint should include
// the $ prefix (e.g. "$ABCDEF0123456789...").
func (c *ControlClient) GetRelayAddress(fingerprint string) (string, error) {
	if err := validateNoNewlines(fingerprint); err != nil {
		return "", fmt.Errorf("tor: relay address: %w", err)
	}
	// Strip the $ prefix for the GETINFO query.
	fp := strings.TrimPrefix(fingerprint, "$")
	info, err := c.GetInfo("ns/id/" + fp)
	if err != nil {
		return "", err
	}
	raw, ok := info["ns/id/"+fp]
	if !ok {
		return "", fmt.Errorf("tor: no network status for %s", fingerprint)
	}
	// Parse the "r" line which contains the IP address.
	// Format: r Nickname Identity Published IP ORPort DirPort
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "r ") {
			fields := strings.Fields(line)
			if len(fields) >= 7 {
				return fields[6], nil
			}
		}
	}
	return "", fmt.Errorf("tor: could not parse address from ns/id/%s", fp)
}

// ResolveRelay resolves a relay path entry to a RelayInfo with country
// and approximate geographic coordinates from the centroid database.
func (c *ControlClient) ResolveRelay(pathEntry string) (*RelayInfo, error) {
	fp, nick := ParseRelayPath(pathEntry)
	ri := &RelayInfo{
		Fingerprint: fp,
		Nickname:    nick,
	}

	ip, err := c.GetRelayAddress(fp)
	if err != nil {
		return ri, err
	}

	cc, err := c.GetRelayCountry(ip)
	if err != nil {
		return ri, err
	}
	ri.CountryCode = cc

	if coords, ok := CountryCentroids[cc]; ok {
		ri.Latitude = coords[0]
		ri.Longitude = coords[1]
	}

	return ri, nil
}
