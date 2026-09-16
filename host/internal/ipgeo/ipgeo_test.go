package ipgeo

import (
	"context"
	"net"
	"testing"
)

func TestIsPrivateOrReserved(t *testing.T) {
	cases := []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.20.0.1", true},
		{"100.64.0.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"114.114.114.114", false},
	}

	for _, c := range cases {
		parsed := net.ParseIP(c.ip)
		if parsed == nil {
			t.Fatalf("failed to parse IP: %s", c.ip)
		}
		got := isPrivateOrReserved(parsed)
		if got != c.expected {
			t.Errorf("IP %s expected isPrivate=%v, got %v", c.ip, c.expected, got)
		}
	}
}

func TestLookupPrivate(t *testing.T) {
	info := Lookup(context.Background(), "192.168.1.254")
	if info.ASOrg != "Private / LAN" {
		t.Errorf("expected Private / LAN, got %s", info.ASOrg)
	}
}
