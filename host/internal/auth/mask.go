package auth

import (
	"fmt"
	"net"
	"strings"
)

// MaskIP masks sensitive parts of IPv4 and IPv6 addresses
func MaskIP(ipStr string) string {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return ""
	}
	parsed := net.ParseIP(ipStr)
	if parsed == nil {
		return "***"
	}
	if ipv4 := parsed.To4(); ipv4 != nil {
		return fmt.Sprintf("%d.%d.***.***", ipv4[0], ipv4[1])
	}
	parts := strings.Split(ipStr, ":")
	if len(parts) >= 4 {
		return fmt.Sprintf("%s:%s:****:****", parts[0], parts[1])
	}
	return "2001:db8:****:****"
}

// MaskHost masks a target host (IP or domain)
func MaskHost(host string) string {
	host = strings.TrimSpace(host)
	if net.ParseIP(host) != nil {
		return MaskIP(host)
	}
	return host
}
