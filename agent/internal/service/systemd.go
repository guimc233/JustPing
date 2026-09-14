package service

import (
	"fmt"
	"os"
	"os/exec"
)

func installSystemd(binPath string) error {
	unit := fmt.Sprintf(`[Unit]
Description=JustPing Agent Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s --config /etc/justping/agent.json
Restart=always
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
`, binPath)

	unitPath := "/etc/systemd/system/justping-agent.service"
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return err
	}

	_ = exec.Command("systemctl", "daemon-reload").Run()
	if err := exec.Command("systemctl", "enable", "--now", "justping-agent").Run(); err != nil {
		return fmt.Errorf("systemctl enable --now failed: %w", err)
	}

	fmt.Println("[Installer] Service installed and started via systemd!")
	return nil
}

func uninstallSystemd() {
	_ = exec.Command("systemctl", "stop", "justping-agent").Run()
	_ = exec.Command("systemctl", "disable", "justping-agent").Run()
	_ = os.Remove("/etc/systemd/system/justping-agent.service")
	_ = exec.Command("systemctl", "daemon-reload").Run()
}
