package service

import (
	"os"
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

// DetectInit detects the true active init system
func DetectInit() InitType {
	if runtime.GOOS == "windows" {
		return InitWindows
	}

	// 1. systemd: only true if running PID 1 is systemd and directory exists
	if _, err := os.Stat("/run/systemd/system"); err == nil {
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
