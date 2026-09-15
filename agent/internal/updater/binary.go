package updater

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// GetBinaryName returns the asset binary name for the current runtime platform.
// Matches naming conventions used in CI and releases, e.g. "justping-agent-linux-amd64".
func GetBinaryName() (string, error) {
	return getBinaryNameFor(runtime.GOOS, runtime.GOARCH)
}

func getBinaryNameFor(goos, goarch string) (string, error) {
	switch goos {
	case "linux":
		switch goarch {
		case "amd64", "386", "arm64", "riscv64", "ppc64le", "s390x", "mips", "mipsle", "mips64", "mips64le":
			return fmt.Sprintf("justping-agent-linux-%s", goarch), nil
		case "arm":
			return fmt.Sprintf("justping-agent-linux-%s", detectLinuxArmArch()), nil
		default:
			return "", fmt.Errorf("unsupported Linux architecture: %s", goarch)
		}
	case "windows":
		switch goarch {
		case "amd64", "386":
			return fmt.Sprintf("justping-agent-windows-%s.exe", goarch), nil
		default:
			return "", fmt.Errorf("unsupported Windows architecture: %s", goarch)
		}
	default:
		return "", fmt.Errorf("unsupported OS: %s", goos)
	}
}

func detectLinuxArmArch() string {
	if out, err := exec.Command("uname", "-m").Output(); err == nil {
		s := strings.TrimSpace(string(out))
		switch {
		case strings.HasPrefix(s, "armv7"):
			return "armv7"
		case strings.HasPrefix(s, "armv6"):
			return "armv6"
		case strings.HasPrefix(s, "armv5"):
			return "armv5"
		}
	}

	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		s := string(data)
		switch {
		case strings.Contains(s, "v7"):
			return "armv7"
		case strings.Contains(s, "v6"):
			return "armv6"
		case strings.Contains(s, "v5"):
			return "armv5"
		}
	}

	return "armv7"
}
