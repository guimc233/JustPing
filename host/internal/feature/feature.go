// Package feature tracks the agent-facing capabilities the Host exposes, together
// with the first probe release that supported each one. The Host reports the
// resulting per-probe support map to the UI so controls backed by a capability a
// probe has not implemented yet can be disabled instead of silently failing.
package feature

import "github.com/guimc233/JustPing/shared/version"

// Key identifies one probe capability.
type Key string

const (
	// Traceroute is the periodic and latency-shift triggered route reporter.
	Traceroute Key = "traceroute"
	// RouteOverride lets the Host disable route tracing per target on one probe.
	RouteOverride Key = "route_override"
	// Proxy lets the Host tunnel HTTPS traffic out through one probe.
	Proxy Key = "proxy"
	// UpdateCheck lets the Host force an immediate self-update check on one probe.
	UpdateCheck Key = "update_check"
)

// Definition describes a capability and the first probe release supporting it.
type Definition struct {
	Key        Key    `json:"key"`
	Name       string `json:"name"`
	MinVersion string `json:"min_version"`
}

// registry is the single source of truth for capability gating. Add a row when a
// new agent-facing capability ships; the Host and the UI pick it up automatically.
// MinVersion values are the release tags that first contained the capability.
var registry = []Definition{
	{Key: Traceroute, Name: "Route tracing", MinVersion: "1.0.7"},
	{Key: RouteOverride, Name: "Per-probe route override", MinVersion: "1.0.10"},
	{Key: Proxy, Name: "HTTPS proxy tunnel", MinVersion: "1.0.12"},
	{Key: UpdateCheck, Name: "Manual update check", MinVersion: "1.0.13"},
}

// All returns every tracked capability in registry order.
func All() []Definition {
	out := make([]Definition, len(registry))
	copy(out, registry)
	return out
}

// MinVersion returns the first probe release supporting key.
func MinVersion(key Key) (string, bool) {
	for _, def := range registry {
		if def.Key == key {
			return def.MinVersion, true
		}
	}
	return "", false
}

// Supports reports whether a probe running agentVersion implements key.
//
// Probes that never reported a version (or reported a placeholder such as
// "unknown"/"dev") are assumed to support everything: a build we cannot identify
// must not have its controls silently disabled.
func Supports(agentVersion string, key Key) bool {
	min, tracked := MinVersion(key)
	if !tracked || version.IsUnknown(min) || version.IsUnknown(agentVersion) {
		return true
	}
	return version.AtLeast(agentVersion, min)
}

// Unsupported returns the keys a probe running agentVersion does not implement,
// in registry order. It returns nil when every tracked capability is supported.
func Unsupported(agentVersion string) []string {
	var out []string
	for _, def := range registry {
		if !Supports(agentVersion, def.Key) {
			out = append(out, string(def.Key))
		}
	}
	return out
}
