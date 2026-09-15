package updater

import (
	"fmt"
	"strings"
)

// DefaultChinaMirrors contains verified GitHub mirror prefixes.
var DefaultChinaMirrors = []string{
	"https://ghfast.top",
	"https://gh-proxy.com",
	"https://ghproxy.net",
	"https://gh.llkk.cc",
	"https://cors.isteed.cc",
}

// GetDownloadCandidates returns a list of candidate base URLs to fetch release assets from.
// When chinaMirror is true, mirror URLs are prioritized followed by direct GitHub.
func GetDownloadCandidates(repo, version string, chinaMirror bool) []string {
	cleanVer := version
	if !strings.HasPrefix(cleanVer, "v") && !strings.HasPrefix(cleanVer, "V") {
		cleanVer = "v" + cleanVer
	}

	releasePath := fmt.Sprintf("/%s/releases/download/%s", repo, cleanVer)
	direct := "https://github.com" + releasePath

	if !chinaMirror {
		return []string{direct}
	}

	var candidates []string
	for _, m := range DefaultChinaMirrors {
		m = strings.TrimRight(m, "/")
		candidates = append(candidates, fmt.Sprintf("%s/https://github.com%s", m, releasePath))
	}
	candidates = append(candidates, direct)
	return candidates
}

// GetLatestCandidates returns a list of candidate URLs to query the latest release from.
func GetLatestCandidates(repo string, chinaMirror bool) []string {
	direct := fmt.Sprintf("https://github.com/%s/releases/latest", repo)
	if !chinaMirror {
		return []string{direct}
	}

	var candidates []string
	for _, m := range DefaultChinaMirrors {
		m = strings.TrimRight(m, "/")
		candidates = append(candidates, fmt.Sprintf("%s/https://github.com/%s/releases/latest", m, repo))
	}
	candidates = append(candidates, direct)
	return candidates
}
