package feature

import "github.com/guimc233/JustPing/shared/version"

// Forcing a restart on a probe that predates the update_check message.
//
// Probes released before UpdateCheck's MinVersion have no way to be asked to
// update over the wire. The lever the Host has on them is a target_sync whose
// numeric fields are out of range, which panics the probe; the service
// supervisor then restarts it and its start-up auto-updater picks up the new
// release. Releases from SoftExit's MinVersion onward instead accept a
// soft_exit message and shut themselves down on request, which is the preferred
// route and what the Host tries first.
//
// The crash is a deliberate, retained mechanism rather than an oversight: the
// probe trusts the Host, and that trust is what makes a wire-level force update
// possible for builds that predate either message. Do not "harden" the
// agent-side clamps without providing a replacement route for these probes.
//
// Both fields are sent in one sync so whichever path fires first wins:
//   - PacketCount overflows make([]SinglePingStat, 0, cap) in
//     agent/internal/pinger/single.go, whose allocation exceeds the runtime
//     limit (~maxAlloc / sizeof(SinglePingStat)).
//   - IntervalSec overflows int64 in time.Duration(sec) * time.Second in
//     agent/cmd/agent/scheduler.go, giving time.NewTicker a negative duration.
const (
	// LegacyCrashMinVersion is the first probe release whose target sync could be
	// crashed this way.
	LegacyCrashMinVersion = "1.0.3"

	// LegacyCrashPacketCount is far past the sliding-window allocation limit.
	LegacyCrashPacketCount = 1 << 50

	// LegacyCrashIntervalSec wraps to a negative time.Duration (~-9.2e18 ns).
	LegacyCrashIntervalSec = 9223372037
)

// ForceRestartMode says how the Host can force a probe to restart itself so its
// start-up auto-updater fetches the latest release. It is the escalation path
// used when a probe cannot be updated through the native update_check message,
// or when an operator explicitly forces a restart.
//
// The ladder, best to worst:
//
//	soft_exit     >= 1.2.1          probe exits on request; supervisor restarts it
//	legacy_crash  1.0.3 .. 1.2.0    probe is crashed; supervisor restarts it
//	unsupported   anything older, or a 32-bit build that cannot carry the payload
//
// This is a separate axis from update_check support: a modern probe supports
// both the native message and soft_exit.
type ForceRestartMode string

const (
	// ForceRestartSoftExit asks the probe to exit so its supervisor restarts it.
	ForceRestartSoftExit ForceRestartMode = "soft_exit"
	// ForceRestartCrash crashes the probe so its supervisor restarts it.
	ForceRestartCrash ForceRestartMode = "legacy_crash"
	// ForceRestartUnsupported means the probe cannot be restarted over the wire.
	ForceRestartUnsupported ForceRestartMode = "unsupported"
)

// sixtyFourBitArchs lists the GOARCH values whose int is 64 bits wide. A 32-bit
// probe cannot carry the out-of-range values at all: encoding/json rejects a
// number that overflows int, the agent drops the whole sync, and nothing
// happens. So the crash route is unavailable on those builds.
var sixtyFourBitArchs = map[string]bool{
	"amd64":    true,
	"arm64":    true,
	"mips64":   true,
	"mips64le": true,
	"ppc64":    true,
	"ppc64le":  true,
	"riscv64":  true,
	"s390x":    true,
}

// ForceRestartModeFor decides how the Host can force agentVersion to restart.
// An unrecognised version is assumed to support soft_exit, matching Supports'
// treatment of probes that never reported a version; soft exit is harmless for a
// probe that does not implement it, because ignoring the message leaves it
// running and the caller escalates to a crash.
func ForceRestartModeFor(agentVersion, agentArch string) ForceRestartMode {
	if Supports(agentVersion, SoftExit) {
		return ForceRestartSoftExit
	}
	if !version.AtLeast(agentVersion, LegacyCrashMinVersion) {
		return ForceRestartUnsupported
	}
	if !sixtyFourBitArchs[agentArch] {
		// The build cannot carry the crash payload.
		return ForceRestartUnsupported
	}
	return ForceRestartCrash
}
