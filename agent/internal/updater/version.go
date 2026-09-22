package updater

import "github.com/guimc233/JustPing/shared/version"

// CleanVersion normalizes version strings like "v1.2.3" -> "1.2.3"
func CleanVersion(v string) string { return version.Clean(v) }

// CompareVersions compares two semver strings (e.g. "1.2.3" and "1.2.4").
// Returns 1 if v1 > v2, -1 if v1 < v2, and 0 if v1 == v2.
func CompareVersions(v1, v2 string) int { return version.Compare(v1, v2) }

// IsNewer reports whether latest is strictly newer than current.
func IsNewer(latest, current string) bool { return version.IsNewer(latest, current) }
