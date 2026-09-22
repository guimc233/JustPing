// Package version provides version-string normalization and comparison shared
// by the Host and the probe agent.
//
// Version strings are semver-like but not strictly semver: releases are tagged
// like "v1.2.3" and the CI injects the tag name verbatim into the binary, so
// consumers must tolerate a leading "v" and the placeholder values "unknown"
// and "dev".
package version

import (
	"strconv"
	"strings"
)

// Placeholder values that carry no comparable version information.
const (
	Unknown = "unknown"
	Dev     = "dev"
)

// Clean normalizes version strings like "v1.2.3" -> "1.2.3".
func Clean(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// IsUnknown reports whether v carries no comparable version information.
func IsUnknown(v string) bool {
	clean := Clean(v)
	return clean == "" || clean == Unknown || clean == Dev
}

// Display renders a version for humans with exactly one leading "v".
//
// Release tags are carried verbatim and already include the prefix ("v1.2.1"),
// while a bare version ("1.2.1") is equally valid input, so callers must not
// prepend "v" themselves or they produce "vv1.2.1". Placeholders such as "dev"
// or "unknown" are returned unchanged rather than decorated into "vdev".
func Display(v string) string {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return ""
	}
	clean := Clean(raw)
	if clean == "" || clean[0] < '0' || clean[0] > '9' {
		return raw
	}
	return "v" + clean
}

// Compare compares two semver strings (e.g. "1.2.3" and "1.2.4").
// Returns 1 if v1 > v2, -1 if v1 < v2, and 0 if v1 == v2.
func Compare(v1, v2 string) int {
	v1 = Clean(v1)
	v2 = Clean(v2)

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

// AtLeast reports whether v is at least min. Values that carry no comparable
// version information (empty, "unknown", "dev") report false so callers can
// decide how to treat them.
func AtLeast(v, min string) bool {
	if IsUnknown(v) || IsUnknown(min) {
		return false
	}
	return Compare(v, min) >= 0
}

// IsNewer reports whether latest is strictly newer than current.
// An unknown current version is treated as older than any known latest.
func IsNewer(latest, current string) bool {
	if Clean(latest) == "" {
		return false
	}
	if IsUnknown(current) {
		return true
	}
	return Compare(latest, current) > 0
}
