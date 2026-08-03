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

// IsBlockedOutboundIP reports whether an address is unsafe as an egress target.
// It is shared by MCP HTTP execution so all runtime HTTP paths enforce the same
// private, reserved, multicast, and unspecified-address policy.
func IsBlockedOutboundIP(ip net.IP) bool {
	return ip == nil || isPrivateIP(ip) || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalMulticast()
}

// ValidateURL rejects URLs that statically target private/reserved address space
// or internal cluster DNS names. DNS resolution and connection pinning are done
// by HTTPToolExecutor's protected transport immediately before dialing.
// P-C220-1: GET SSRF. P-C221-1: POST SSRF.
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
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

	return nil
}
