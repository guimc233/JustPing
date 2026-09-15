package service

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type AgentConfig struct {
	Server      string `json:"server"`
	Token       string `json:"token"`
	ChinaMirror bool   `json:"china_mirror"`
	AutoUpdate  *bool  `json:"auto_update,omitempty"`
}

// IsAutoUpdateEnabled returns true if AutoUpdate is nil (default: enabled) or explicitly set to true.
func (c AgentConfig) IsAutoUpdateEnabled() bool {
	if c.AutoUpdate == nil {
		return true
	}
	return *c.AutoUpdate
}

// InstallService writes configuration and installs the agent as an OS service
func InstallService(binPath string, cfg AgentConfig) error {
	initType := DetectInit()
	fmt.Printf("[Installer] Detected init system: %s\n", initType)

	if runtime.GOOS == "windows" {
		return installWindows(binPath, cfg)
	}

	if err := os.MkdirAll("/etc/justping", 0755); err != nil {
		return fmt.Errorf("failed to create /etc/justping: %w", err)
	}

	cfgData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format config: %w", err)
	}

	if err := os.WriteFile("/etc/justping/agent.json", cfgData, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	targetBin := "/usr/local/bin/justping-agent"
	if _, err := os.Stat("/usr/local/bin"); err != nil {
		targetBin = "/usr/bin/justping-agent"
	}

	absBin, _ := filepath.Abs(binPath)
	if absBin != targetBin {
		if err := copyFile(absBin, targetBin); err != nil {
			return fmt.Errorf("failed to copy binary to %s: %w", targetBin, err)
		}
		_ = os.Chmod(targetBin, 0755)
	}

	_ = exec.Command("setcap", "cap_net_raw+ep", targetBin).Run()

	switch initType {
	case InitSystemd:
		return installSystemd(targetBin)
	case InitOpenRC:
		return installOpenRC(targetBin)
	case InitProcd:
		return installProcd(targetBin)
	case InitRunit:
		return installRunit(targetBin)
	case InitSysVinit:
		return installSysVinit(targetBin)
	default:
		return fmt.Errorf("unsupported or unknown init system: %s", initType)
	}
}

// UninstallService stops and removes the installed service
func UninstallService() error {
	initType := DetectInit()
	fmt.Printf("[Installer] Uninstalling service for %s\n", initType)

	switch initType {
	case InitSystemd:
		uninstallSystemd()
	case InitOpenRC:
		uninstallOpenRC()
	case InitProcd:
		_ = exec.Command("/etc/init.d/justping-agent", "stop").Run()
		_ = exec.Command("/etc/init.d/justping-agent", "disable").Run()
		_ = os.Remove("/etc/init.d/justping-agent")
	case InitRunit:
		_ = exec.Command("sv", "down", "justping-agent").Run()
		_ = os.Remove("/var/service/justping-agent")
		_ = os.RemoveAll("/etc/sv/justping-agent")
	case InitSysVinit:
		_ = exec.Command("/etc/init.d/justping-agent", "stop").Run()
		if _, err := exec.LookPath("update-rc.d"); err == nil {
			_ = exec.Command("update-rc.d", "-f", "justping-agent", "remove").Run()
		} else if _, err := exec.LookPath("chkconfig"); err == nil {
			_ = exec.Command("chkconfig", "--del", "justping-agent").Run()
		}
		_ = os.Remove("/etc/init.d/justping-agent")
	}

	_ = os.RemoveAll("/etc/justping")
	_ = os.Remove("/usr/local/bin/justping-agent")
	_ = os.Remove("/usr/bin/justping-agent")
	_ = os.Remove("/run/justping-agent.pid")
	_ = os.Remove("/var/run/justping-agent.pid")
	fmt.Println("[Installer] JustPing agent uninstalled successfully.")
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
