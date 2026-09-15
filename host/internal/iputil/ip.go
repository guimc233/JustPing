package iputil

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

// CleanIP trims whitespace, surrounding quotes or brackets,
// strips port if present, and returns the normalized IP string, or "" if invalid.
func CleanIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Strip surrounding quotes
	raw = strings.Trim(raw, `"'`)

	// Try net.SplitHostPort first (handles "1.2.3.4:8080", "[2001:db8::1]:8080")
	if host, port, err := net.SplitHostPort(raw); err == nil {
		if port != "" {
			p, err := strconv.Atoi(port)
			if err != nil || p < 1 || p > 65535 {
				return ""
			}
		}
		raw = host
	}

	raw = strings.TrimSpace(raw)
	// Strip IPv6 brackets if present without port: "[2001:db8::1]"
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	raw = strings.TrimSpace(raw)

	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}

	return ip.String()
}

// IsPublicIP reports whether ip is a public IP address (not nil, loopback, private,
// link-local, unspecified, or multicast).
func IsPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	return true
}

// ExtractXForwardedFor extracts the best client IP from a comma-separated X-Forwarded-For header value.
// It prioritizes the first valid public IP; if no public IP is present, it returns the first valid IP.
func ExtractXForwardedFor(headerVal string) string {
	ip, _ := extractFromXFFWithStatus(headerVal)
	return ip
}

func extractFromXFFWithStatus(headerVal string) (ipStr string, isPublic bool) {
	parts := strings.Split(headerVal, ",")
	var firstValid string
	for _, part := range parts {
		cleaned := CleanIP(part)
		if cleaned == "" {
			continue
		}
		ip := net.ParseIP(cleaned)
		if ip == nil {
			continue
		}
		if IsPublicIP(ip) {
			return cleaned, true
		}
		if firstValid == "" {
			firstValid = cleaned
		}
	}
	return firstValid, false
}

// GetClientIP extracts the client's real IP address from an http.Request.
// It checks headers in order of priority:
// 1. CF-Connecting-IP (Cloudflare)
// 2. X-Forwarded-For (first public IP, or first valid)
// 3. X-Real-IP
// 4. RemoteAddr
func GetClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	// 1. CF-Connecting-IP (Cloudflare)
	if cfIP := CleanIP(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		return cfIP
	}

	// 2. X-Forwarded-For
	var xffFallback string
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip, isPublic := extractFromXFFWithStatus(xff); ip != "" {
			if isPublic {
				return ip
			}
			xffFallback = ip
		}
	}

	// 3. X-Real-IP
	if realIP := CleanIP(r.Header.Get("X-Real-IP")); realIP != "" {
		if parsed := net.ParseIP(realIP); parsed != nil && IsPublicIP(parsed) {
			return realIP
		}
		if xffFallback == "" {
			xffFallback = realIP
		}
	}

	// Return fallback from X-Forwarded-For or X-Real-IP if found
	if xffFallback != "" {
		return xffFallback
	}

	// 4. RemoteAddr fallback
	if r.RemoteAddr != "" {
		if ip := CleanIP(r.RemoteAddr); ip != "" {
			return ip
		}
		return r.RemoteAddr
	}

	return ""
}
