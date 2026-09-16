package ipgeo

import (
	"regexp"
	"strings"

	"github.com/guimc233/JustPing/host/internal/model"
)

// HopMeta provides the IP and ASN information needed for route classification.
type HopMeta struct {
	IP       string
	ASNumber string
	ASOrg    string
	ISP      string
}

// ClassifyRoute classifies the network route based on the algorithm from TcpQuality (runTcpQuality-core.sh).
// It identifies prestigious and standard Chinese backbone routes such as CN2 GIA, CN2 GT, 163, 9929, 10099, 4837, CMIN2, CMI, CERNET.
func ClassifyRoute(hops []HopMeta, targetHost, targetISP string) string {
	if len(hops) == 0 {
		return "Unknown"
	}

	var asns []string
	var ips []string

	for _, h := range hops {
		ip := strings.TrimSpace(h.IP)
		if ip == "" || ip == "*" {
			continue
		}
		asn := cleanASN(h.ASNumber)
		if asn == "" {
			asn = inferASNFromIP(ip)
		}
		asns = append(asns, asn)
		ips = append(ips, ip)
	}

	if len(ips) == 0 {
		return "Timeout / Hidden"
	}

	// 1. Telecom Analysis (电信)
	hasCN2 := false
	hasCN2Core := false
	has163 := false
	hasCTGNet := false
	cn2HopIndex := -1
	last163Index := -1

	for i, ip := range ips {
		asn := asns[i]
		if isCN2IP(ip) || asn == "4809" {
			hasCN2 = true
			if strings.HasPrefix(ip, "59.43.") {
				hasCN2Core = true
			}
			if cn2HopIndex == -1 {
				cn2HopIndex = i
			}
		}
		if isCTGNetIP(ip) || asn == "23764" {
			hasCTGNet = true
		}
		if is163IP(ip) || asn == "4134" || asn == "4847" {
			has163 = true
			last163Index = i
		}
	}

	if hasCN2 {
		if has163 && last163Index > cn2HopIndex {
			return "电信 CN2 GT"
		}
		if hasCN2Core {
			return "电信 CN2 GIA"
		}
		return "电信 CN2"
	}
	if hasCTGNet {
		if has163 {
			return "电信 CTGNet->163"
		}
		return "电信 CTGNet"
	}
	if has163 {
		return "电信 163"
	}

	// 2. Unicom Analysis (联通)
	has9929 := false
	has10099 := false
	has4837 := false
	first10099 := -1
	first9929 := -1
	first4837 := -1

	for i, ip := range ips {
		asn := asns[i]
		if is9929IP(ip) || asn == "9929" {
			has9929 = true
			if first9929 == -1 {
				first9929 = i
			}
		}
		if is10099IP(ip) || asn == "10099" {
			has10099 = true
			if first10099 == -1 {
				first10099 = i
			}
		}
		if is4837IP(ip) || asn == "4837" || asn == "4808" {
			has4837 = true
			if first4837 == -1 {
				first4837 = i
			}
		}
	}

	if has10099 {
		if has9929 && first9929 > first10099 {
			return "联通 10099->9929"
		}
		if has4837 && first4837 > first10099 {
			return "联通 10099->4837"
		}
		return "联通 10099"
	}
	if has9929 {
		return "联通 9929 (A网)"
	}
	if has4837 {
		return "联通 4837 (169)"
	}

	// 3. Mobile Analysis (移动)
	hasCMIN2 := false
	hasCMI := false
	hasCMNET := false
	firstCMIN2 := -1
	firstCMI := -1

	for i, ip := range ips {
		asn := asns[i]
		if isCMIN2IP(ip) || asn == "58807" {
			hasCMIN2 = true
			if firstCMIN2 == -1 {
				firstCMIN2 = i
			}
		}
		if isCMIIP(ip) || asn == "58453" {
			hasCMI = true
			if firstCMI == -1 {
				firstCMI = i
			}
		}
		if isCMNETIP(ip) || asn == "9808" {
			hasCMNET = true
		}
	}

	if hasCMIN2 {
		if hasCMI && firstCMI > firstCMIN2 {
			return "移动 CMIN2->CMI"
		}
		if hasCMNET {
			return "移动 CMIN2->CMNET"
		}
		return "移动 CMIN2"
	}
	if hasCMI {
		if hasCMNET {
			return "移动 CMI->CMNET"
		}
		return "移动 CMI"
	}
	if hasCMNET {
		return "移动 CMNET"
	}

	// 4. Education Network (CERNET / CERNET2)
	for i, ip := range ips {
		asn := asns[i]
		if asn == "4538" || strings.HasPrefix(ip, "202.112.") || strings.HasPrefix(ip, "101.4.") {
			return "教育网 CERNET"
		}
		if asn == "23910" || asn == "23911" || strings.HasPrefix(ip, "2001:da8:") || strings.HasPrefix(ip, "2001:250:") {
			return "教育网 CERNET2"
		}
	}

	// 5. Common Global Carriers / Upstream Transits
	for _, asn := range asns {
		switch asn {
		case "1299":
			return "Arelion (Telia)"
		case "2914":
			return "NTT"
		case "174":
			return "Cogent"
		case "3257":
			return "GTT"
		case "6453":
			return "Tata Communications"
		case "3356":
			return "Lumen (Level3)"
		case "6939":
			return "Hurricane Electric (HE)"
		case "13335":
			return "Cloudflare"
		}
	}

	return "Standard IP Transit"
}

func inferASNFromIP(ip string) string {
	if strings.HasPrefix(ip, "59.43.") {
		return "4809"
	}
	if strings.HasPrefix(ip, "203.22.182.") || strings.HasPrefix(ip, "203.22.178.") || strings.HasPrefix(ip, "203.128.224.") {
		return "23764"
	}
	if strings.HasPrefix(ip, "202.97.") || strings.HasPrefix(ip, "202.96.") || strings.HasPrefix(ip, "219.141.") || strings.HasPrefix(ip, "240e:") {
		return "4134"
	}
	if strings.HasPrefix(ip, "219.158.") || strings.HasPrefix(ip, "2408:") {
		return "4837"
	}
	if strings.HasPrefix(ip, "223.120.") || strings.HasPrefix(ip, "223.119.") {
		return "58453"
	}
	if strings.HasPrefix(ip, "221.183.") || strings.HasPrefix(ip, "111.24.") || strings.HasPrefix(ip, "2409:8080:") {
		return "9808"
	}
	if strings.HasPrefix(ip, "2402:4f00:f000:") {
		return "58807"
	}
	if is10099IP(ip) {
		return "10099"
	}
	if is9929IP(ip) {
		return "9929"
	}
	if strings.HasPrefix(ip, "101.4.") || strings.HasPrefix(ip, "202.112.") {
		return "4538"
	}
	return ""
}

func cleanASN(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "AS")
	raw = strings.TrimPrefix(raw, "as")
	return raw
}

func isCN2IP(ip string) bool {
	return strings.HasPrefix(ip, "59.43.") || strings.HasPrefix(ip, "2605:9d80:")
}

func isCTGNetIP(ip string) bool {
	return strings.HasPrefix(ip, "203.22.182.") || strings.HasPrefix(ip, "203.22.178.") ||
		strings.HasPrefix(ip, "203.22.179.") || strings.HasPrefix(ip, "203.128.224.") ||
		strings.HasPrefix(ip, "69.194.") || strings.HasPrefix(ip, "2400:9380:")
}

func is163IP(ip string) bool {
	return strings.HasPrefix(ip, "202.97.") || strings.HasPrefix(ip, "202.96.") ||
		strings.HasPrefix(ip, "219.141.") || strings.HasPrefix(ip, "219.142.") ||
		strings.HasPrefix(ip, "106.37.") || strings.HasPrefix(ip, "240e:")
}

func is9929IP(ip string) bool {
	return strings.HasPrefix(ip, "210.14.") || strings.HasPrefix(ip, "210.51.") ||
		strings.HasPrefix(ip, "210.78.") || strings.HasPrefix(ip, "218.105.")
}

var re10099 = regexp.MustCompile(`^(103\.214\.|103\.228\.68\.|103\.239\.176\.|118\.26\.151\.|162\.219\.(3[2-9]|85)\.|162\.245\.124\.|202\.77\.23\.|203\.160\.(66|75)\.|2401:8a00:)`)

func is10099IP(ip string) bool {
	return re10099.MatchString(ip)
}

func is4837IP(ip string) bool {
	return strings.HasPrefix(ip, "219.158.") || strings.HasPrefix(ip, "2408:")
}

func isCMIN2IP(ip string) bool {
	return strings.HasPrefix(ip, "2402:4f00:f000:")
}

func isCMIIP(ip string) bool {
	return strings.HasPrefix(ip, "223.120.") || strings.HasPrefix(ip, "223.119.")
}

func isCMNETIP(ip string) bool {
	return strings.HasPrefix(ip, "221.183.") || strings.HasPrefix(ip, "111.24.") ||
		strings.HasPrefix(ip, "111.13.") || strings.HasPrefix(ip, "2409:8080:")
}

type ASNode = model.ASNode

// GenerateASPath extracts an ordered, deduplicated sequence of Autonomous Systems with organization names.
// e.g. "AS13335 (Cloudflare) -> AS58453 (CMI) -> AS9808 (China Mobile)"
func GenerateASPath(hops []HopMeta) (string, []ASNode) {
	var nodes []ASNode
	var lastASN string

	for _, h := range hops {
		ip := strings.TrimSpace(h.IP)
		if ip == "" || ip == "*" {
			continue
		}

		asn := cleanASN(h.ASNumber)
		if asn == "" {
			asn = inferASNFromIP(ip)
		}

		// Skip private / unknown hops
		if asn == "" || asn == "*" || asn == "0" {
			continue
		}

		fullASN := "AS" + asn
		if fullASN == lastASN {
			continue
		}

		name := simplifyASName(asn, h.ASOrg, h.ISP)
		nodes = append(nodes, ASNode{
			ASN:  fullASN,
			Name: name,
		})
		lastASN = fullASN
	}

	if len(nodes) == 0 {
		return "", nil
	}

	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Name != "" {
			parts = append(parts, n.ASN+" ("+n.Name+")")
		} else {
			parts = append(parts, n.ASN)
		}
	}

	return strings.Join(parts, " -> "), nodes
}

func simplifyASName(asn, org, isp string) string {
	switch asn {
	case "4809":
		return "CN2"
	case "4134":
		return "Chinanet 163"
	case "23764":
		return "CTGNet"
	case "9929":
		return "China Unicom 9929"
	case "10099":
		return "CUG 10099"
	case "4837":
		return "China Unicom 4837"
	case "58807":
		return "CMIN2"
	case "58453":
		return "CMI"
	case "9808":
		return "China Mobile"
	case "4538":
		return "CERNET"
	case "23910":
		return "CERNET2"
	case "23911":
		return "CERNET2"
	case "13335":
		return "Cloudflare"
	case "15169":
		return "Google"
	case "8075":
		return "Microsoft"
	case "16509":
		return "Amazon AWS"
	case "20940":
		return "Akamai"
	case "54113":
		return "Fastly"
	case "1299":
		return "Arelion"
	case "2914":
		return "NTT"
	case "174":
		return "Cogent"
	case "3257":
		return "GTT"
	case "6453":
		return "Tata"
	case "3356":
		return "Lumen"
	case "6939":
		return "HE"
	}

	raw := org
	if raw == "" {
		raw = isp
	}
	raw = strings.TrimSpace(raw)
	if len(raw) > 20 {
		// Cut long org name
		fields := strings.Fields(raw)
		if len(fields) > 0 {
			raw = fields[0]
		}
	}
	return raw
}
