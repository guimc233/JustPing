package ipgeo

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type GeoInfo struct {
	IP          string `json:"ip"`
	ASNumber    string `json:"as_number"`
	ASOrg       string `json:"as_org"`
	ISP         string `json:"isp"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
}

var (
	cache      sync.Map // ip -> GeoInfo
	httpClient = &http.Client{Timeout: 3 * time.Second}
)

// Lookup retrieves ASN & Geo information for an IP, following NextTrace data patterns.
func Lookup(ctx context.Context, ipStr string) GeoInfo {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" || ipStr == "*" {
		return GeoInfo{IP: ipStr}
	}

	// Check in-memory cache
	if cached, ok := cache.Load(ipStr); ok {
		return cached.(GeoInfo)
	}

	parsed := net.ParseIP(ipStr)
	if parsed == nil {
		return GeoInfo{IP: ipStr}
	}

	// If private or loopback or link-local or carrier-grade NAT
	if isPrivateOrReserved(parsed) {
		info := GeoInfo{
			IP:       ipStr,
			ASNumber: "*",
			ASOrg:    "Private / LAN",
			ISP:      "Local Network",
			Country:  "Local",
		}
		cache.Store(ipStr, info)
		return info
	}

	info := queryNextTrace(ctx, ipStr)
	if info.ASNumber == "" && info.ISP == "" {
		// Fallback to IP-API
		info = queryIPApi(ctx, ipStr)
	}

	info.IP = ipStr
	cache.Store(ipStr, info)
	return info
}

// NextTrace API schema (api.nxtrace.org or api.leo.moe)
type nextTraceResponse struct {
	Asnumber    string `json:"asnumber"`
	Owner       string `json:"owner"`
	Isp         string `json:"isp"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
}

func queryNextTrace(ctx context.Context, ipStr string) GeoInfo {
	// Query NextTrace API
	url := fmt.Sprintf("https://api.nxtrace.org/v1/ip/%s", ipStr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return GeoInfo{}
	}
	req.Header.Set("User-Agent", "NextTrace-Core/1.0 (JustPing)")

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		// Try LeoMoe mirror
		urlLeo := fmt.Sprintf("https://api.leo.moe/ip?ip=%s", ipStr)
		reqLeo, errLeo := http.NewRequestWithContext(ctx, http.MethodGet, urlLeo, nil)
		if errLeo != nil {
			return GeoInfo{}
		}
		reqLeo.Header.Set("User-Agent", "NextTrace-Core/1.0 (JustPing)")
		respLeo, errDo := httpClient.Do(reqLeo)
		if errDo != nil || respLeo.StatusCode != http.StatusOK {
			if respLeo != nil {
				_ = respLeo.Body.Close()
			}
			return GeoInfo{}
		}
		defer respLeo.Body.Close()
		var nxt nextTraceResponse
		if err := json.NewDecoder(respLeo.Body).Decode(&nxt); err == nil && (nxt.Asnumber != "" || nxt.Owner != "") {
			return GeoInfo{
				ASNumber:    nxt.Asnumber,
				ASOrg:       nxt.Owner,
				ISP:         nxt.Isp,
				Country:     nxt.Country,
				CountryCode: nxt.CountryCode,
				City:        nxt.City,
			}
		}
		return GeoInfo{}
	}
	defer resp.Body.Close()

	var nxt nextTraceResponse
	if err := json.NewDecoder(resp.Body).Decode(&nxt); err == nil {
		return GeoInfo{
			ASNumber:    nxt.Asnumber,
			ASOrg:       nxt.Owner,
			ISP:         nxt.Isp,
			Country:     nxt.Country,
			CountryCode: nxt.CountryCode,
			City:        nxt.City,
		}
	}
	return GeoInfo{}
}

// Fallback IP-API schema
type ipApiResponse struct {
	Status      string `json:"status"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	City        string `json:"city"`
	ISP         string `json:"isp"`
	Org         string `json:"org"`
	AS          string `json:"as"`
}

func queryIPApi(ctx context.Context, ipStr string) GeoInfo {
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,countryCode,city,isp,org,as", ipStr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return GeoInfo{}
	}

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return GeoInfo{}
	}
	defer resp.Body.Close()

	var r ipApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil || r.Status != "success" {
		return GeoInfo{}
	}

	asNum := ""
	if parts := strings.Fields(r.AS); len(parts) > 0 && strings.HasPrefix(parts[0], "AS") {
		asNum = parts[0]
	}

	return GeoInfo{
		ASNumber:    asNum,
		ASOrg:       r.Org,
		ISP:         r.ISP,
		Country:     r.Country,
		CountryCode: r.CountryCode,
		City:        r.City,
	}
}

func isPrivateOrReserved(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		// Carrier-grade NAT 100.64.0.0/10
		if ip4[0] == 100 && (ip4[1] >= 64 && ip4[1] <= 127) {
			return true
		}
	}
	return false
}
