package service

import (
	"os"
	"os/exec"
	"runtime"
)

type InitType string

const (
	InitSystemd  InitType = "systemd"
	InitOpenRC   InitType = "openrc"
	InitProcd    InitType = "procd"
	InitRunit    InitType = "runit"
	InitSysVinit InitType = "sysvinit"
	InitWindows  InitType = "windows"
	InitUnknown  InitType = "unknown"
)

// DetectInit detects the Linux init system in use
func DetectInit() InitType {
	if runtime.GOOS == "windows" {
		return InitWindows
	}

	// 1. systemd
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return InitSystemd
	}
	if p, err := exec.LookPath("systemctl"); err == nil && p != "" {
		return InitSystemd
	}

	// 2. OpenWrt procd
	if _, err := os.Stat("/etc/openwrt_release"); err == nil {
		return InitProcd
	}

	// 3. OpenRC (Alpine, Gentoo)
	if _, err := os.Stat("/run/openrc"); err == nil {
		return InitOpenRC
	}
	if _, err := os.Stat("/sbin/openrc-run"); err == nil {
		return InitOpenRC
	}

	// 4. runit (Void Linux)
	if _, err := os.Stat("/etc/sv"); err == nil {
		return InitRunit
	}

	// 5. SysVinit fallback
	if _, err := os.Stat("/etc/init.d"); err == nil {
		return InitSysVinit
	}

	return InitUnknown
}
