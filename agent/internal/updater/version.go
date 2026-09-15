package updater

import (
	"strconv"
	"strings"
)

// CleanVersion normalizes version strings like "v1.2.3" -> "1.2.3"
func CleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// CompareVersions compares two semver strings (e.g. "1.2.3" and "1.2.4").
// Returns 1 if v1 > v2, -1 if v1 < v2, and 0 if v1 == v2.
func CompareVersions(v1, v2 string) int {
	v1 = CleanVersion(v1)
	v2 = CleanVersion(v2)

	if v1 == v2 {
		return 0
	}

	// Separate pre-release info (e.g. 1.0.0-rc1)
	core1 := strings.Split(v1, "-")[0]
	core2 := strings.Split(v2, "-")[0]

	parts1 := strings.Split(core1, ".")
	parts2 := strings.Split(core2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	// If core versions are identical, release without pre-release is newer than one with pre-release
	hasPre1 := strings.Contains(v1, "-")
	hasPre2 := strings.Contains(v2, "-")
	if hasPre1 && !hasPre2 {
		return -1
	}
	if !hasPre1 && hasPre2 {
		return 1
	}

	return strings.Compare(v1, v2)
}

// IsNewer reports whether latest is strictly newer than current.
func IsNewer(latest, current string) bool {
	cleanLatest := CleanVersion(latest)
	cleanCurrent := CleanVersion(current)

	if cleanLatest == "" {
		return false
	}
	if cleanCurrent == "" || cleanCurrent == "unknown" || cleanCurrent == "dev" {
		return true
	}
	return CompareVersions(cleanLatest, cleanCurrent) > 0
}
