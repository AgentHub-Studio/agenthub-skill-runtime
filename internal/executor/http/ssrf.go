package http

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// privateCIDRs is the blocklist of address ranges that HTTP tools must not target.
// P-C220-1 / P-C221-1: prevents SSRF attacks via crafted tool URLs.
var privateCIDRs []*net.IPNet

func init() {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16", // link-local
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 ULA
		"100.64.0.0/10",  // CGNAT (RFC 6598)
	}
	for _, cidr := range cidrs {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil {
			privateCIDRs = append(privateCIDRs, network)
		}
	}
}

// isPrivateIP returns true when ip falls within any of the private/reserved CIDRs.
func isPrivateIP(ip net.IP) bool {
	for _, cidr := range privateCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateURL rejects URLs that target private/reserved address space or internal
// cluster DNS names. Fail-closed: DNS resolution failure is treated as a block.
// P-C220-1: GET SSRF. P-C221-1: POST SSRF.
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Host == "" {
		return fmt.Errorf("URL has no host")
	}

	host := u.Hostname()

	// Block internal cluster DNS names before any resolution.
	if strings.HasSuffix(host, ".svc.cluster.local") ||
		strings.HasSuffix(host, ".cluster.local") {
		return fmt.Errorf("URL targets internal cluster DNS: %s", host)
	}

	// If the host is already an IP, check directly (no DNS needed).
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("URL targets private/reserved network address: %s", host)
		}
		return nil
	}

	// Resolve hostname and check each returned address.
	// Fail-closed: unresolvable hosts are rejected.
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("DNS resolution failed for %s: %w", host, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) {
			return fmt.Errorf("URL targets private/internal network address %s (resolved from %s)", addr, host)
		}
	}
	return nil
}
