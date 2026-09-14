package service

import (
	"fmt"
	"os"
	"os/exec"
)

func installOpenRC(binPath string) error {
	script := fmt.Sprintf(`#!/sbin/openrc-run
description="JustPing Agent Service"

command="%s"
command_args="--config /etc/justping/agent.json"
command_background="yes"
pidfile="/run/justping-agent.pid"

depend() {
    need net
    after firewall
}
`, binPath)

	scriptPath := "/etc/init.d/justping-agent"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return err
	}

	_ = exec.Command("rc-update", "add", "justping-agent", "default").Run()
	_ = exec.Command("rc-service", "justping-agent", "restart").Run()
	fmt.Println("[Installer] Service installed and started via OpenRC!")
	return nil
}

func uninstallOpenRC() {
	_ = exec.Command("rc-service", "justping-agent", "stop").Run()
	_ = exec.Command("rc-update", "del", "justping-agent", "default").Run()
	_ = os.Remove("/etc/init.d/justping-agent")
}
