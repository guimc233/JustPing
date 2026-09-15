package iputil

import (
	"net"
	"net/http"
	"testing"
)

func TestCleanIP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"standard ipv4", "192.0.2.1", "192.0.2.1"},
		{"ipv4 with spaces", "  192.0.2.1  ", "192.0.2.1"},
		{"ipv4 with port", "192.0.2.1:8080", "192.0.2.1"},
		{"ipv4 with quotes", `"192.0.2.1"`, "192.0.2.1"},
		{"ipv4 with single quotes", `'192.0.2.1'`, "192.0.2.1"},
		{"standard ipv6", "2001:db8::1", "2001:db8::1"},
		{"ipv6 bracketed", "[2001:db8::1]", "2001:db8::1"},
		{"ipv6 bracketed with port", "[2001:db8::1]:443", "2001:db8::1"},
		{"invalid ip string", "invalid.ip", ""},
		{"invalid port format", "192.0.2.1:999999", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanIP(tt.input)
			if result != tt.expected {
				t.Errorf("CleanIP(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"nil ip", "", false},
		{"public ipv4", "8.8.8.8", true},
		{"public ipv4 benchmark", "203.0.113.1", true},
		{"private 10/8", "10.0.0.1", false},
		{"private 172.16/12", "172.16.0.1", false},
		{"private 192.168/16", "192.168.1.1", false},
		{"loopback ipv4", "127.0.0.1", false},
		{"unspecified ipv4", "0.0.0.0", false},
		{"link local ipv4", "169.254.1.1", false},
		{"multicast ipv4", "224.0.0.1", false},
		{"public ipv6", "2001:db8::1", true},
		{"loopback ipv6", "::1", false},
		{"unspecified ipv6", "::", false},
		{"private ula ipv6", "fc00::1", false},
		{"link local ipv6", "fe80::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parsed net.IP
			if tt.ip != "" {
				parsed = net.ParseIP(tt.ip)
			}
			result := IsPublicIP(parsed)
			if result != tt.expected {
				t.Errorf("IsPublicIP(%v) = %v, expected %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestExtractXForwardedFor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"single public ip", "203.0.113.5", "203.0.113.5"},
		{"single private ip", "192.168.1.1", "192.168.1.1"},
		{"single public with port", "203.0.113.5:12345", "203.0.113.5"},
		{"client public, proxy private", "203.0.113.5, 10.0.0.1", "203.0.113.5"},
		{"client private, proxy public (traversing proxy)", "10.0.0.1, 203.0.113.5", "203.0.113.5"},
		{"client public, proxy public", "203.0.113.5, 198.51.100.2", "203.0.113.5"},
		{"internal proxies only", "10.0.0.1, 192.168.1.1, 172.16.0.5", "10.0.0.1"},
		{"with junk entry", "unknown, 203.0.113.5, 10.0.0.1", "203.0.113.5"},
		{"all junk", "unknown, invalid", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractXForwardedFor(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractXForwardedFor(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	if ip := GetClientIP(nil); ip != "" {
		t.Errorf("GetClientIP(nil) should be empty, got %q", ip)
	}

	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	// 1. RemoteAddr only
	if ip := GetClientIP(req); ip != "127.0.0.1" {
		t.Errorf("Expected RemoteAddr 127.0.0.1, got %q", ip)
	}

	// 2. X-Real-IP takes precedence over RemoteAddr
	req.Header.Set("X-Real-IP", "198.51.100.1")
	if ip := GetClientIP(req); ip != "198.51.100.1" {
		t.Errorf("Expected X-Real-IP 198.51.100.1, got %q", ip)
	}

	// 3. X-Forwarded-For takes precedence over X-Real-IP
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	if ip := GetClientIP(req); ip != "203.0.113.1" {
		t.Errorf("Expected X-Forwarded-For 203.0.113.1, got %q", ip)
	}

	// 4. CF-Connecting-IP takes highest precedence
	req.Header.Set("CF-Connecting-IP", "192.0.2.1")
	if ip := GetClientIP(req); ip != "192.0.2.1" {
		t.Errorf("Expected CF-Connecting-IP 192.0.2.1, got %q", ip)
	}

	// 5. XFF private but X-Real-IP public -> prefers public from X-Real-IP
	req2, _ := http.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "172.17.0.1:8080"
	req2.Header.Set("X-Forwarded-For", "10.0.0.1")
	req2.Header.Set("X-Real-IP", "203.0.113.50")
	if ip := GetClientIP(req2); ip != "203.0.113.50" {
		t.Errorf("Expected X-Real-IP public IP 203.0.113.50, got %q", ip)
	}
}
