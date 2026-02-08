package api

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// ipAllowlistMiddleware restricts access to allowed IP addresses/CIDR ranges.
// Returns 403 Forbidden if the client IP is not in the allowlist.
func ipAllowlistMiddleware(allowedCIDRs []string) func(http.Handler) http.Handler {
	// Parse CIDR ranges at initialization time
	cidrs := make([]*net.IPNet, 0, len(allowedCIDRs))
	for _, cidr := range allowedCIDRs {
		// Handle both CIDR notation and single IPs
		if !strings.Contains(cidr, "/") {
			// Single IP - add /32 for IPv4 or /128 for IPv6
			if strings.Contains(cidr, ":") {
				cidr = cidr + "/128" // IPv6
			} else {
				cidr = cidr + "/32" // IPv4
			}
		}

		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			// Log error but continue - invalid CIDRs are skipped
			continue
		}
		cidrs = append(cidrs, ipnet)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no CIDRs configured, allow all (fail open for development)
			if len(cidrs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIPStr := getClientIP(r)
			clientIP := net.ParseIP(clientIPStr)
			if clientIP == nil {
				respondError(w, r, http.StatusForbidden, "invalid_ip", 
					"Unable to determine client IP address")
				return
			}

			// Check if IP is in any allowed CIDR range
			allowed := false
			for _, cidr := range cidrs {
				if cidr.Contains(clientIP) {
					allowed = true
					break
				}
			}

			if !allowed {
				respondError(w, r, http.StatusForbidden, "ip_not_allowed",
					fmt.Sprintf("IP address %s is not authorized for this operation", clientIPStr))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
