package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func installProcd(binPath string) error {
	script := fmt.Sprintf(`#!/bin/sh /etc/rc.common
USE_PROCD=1
START=99
STOP=10

start_service() {
    procd_open_instance
    procd_set_param command %s --config /etc/justping/agent.json
    procd_set_param respawn
    procd_close_instance
}
`, binPath)

	scriptPath := "/etc/init.d/justping-agent"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return err
	}

	_ = exec.Command("/etc/init.d/justping-agent", "enable").Run()
	_ = exec.Command("/etc/init.d/justping-agent", "restart").Run()
	fmt.Println("[Installer] Service installed and started via OpenWrt procd!")
	return nil
}

func installRunit(binPath string) error {
	svDir := "/etc/sv/justping-agent"
	if err := os.MkdirAll(svDir, 0755); err != nil {
		return err
	}

	runScript := fmt.Sprintf(`#!/bin/sh
exec 2>&1
exec %s --config /etc/justping/agent.json
`, binPath)

	if err := os.WriteFile(filepath.Join(svDir, "run"), []byte(runScript), 0755); err != nil {
		return err
	}

	if _, err := os.Stat("/var/service"); err == nil {
		_ = os.Symlink(svDir, "/var/service/justping-agent")
		fmt.Println("[Installer] Service linked in /var/service via runit!")
	}
	return nil
}

func installSysVinit(binPath string) error {
	script := fmt.Sprintf(`#!/bin/sh
### BEGIN INIT INFO
# Provides:          justping-agent
# Required-Start:    $network $remote_fs $syslog
# Required-Stop:     $network $remote_fs $syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: JustPing Agent Daemon
### END INIT INFO

NAME=justping-agent
DAEMON="%s"
DAEMON_ARGS="--config /etc/justping/agent.json"
PIDFILE=/var/run/$NAME.pid

case "$1" in
  start)
    echo "Starting $NAME..."
    start-stop-daemon --start --background --make-pidfile --pidfile $PIDFILE --exec $DAEMON -- $DAEMON_ARGS
    ;;
  stop)
    echo "Stopping $NAME..."
    start-stop-daemon --stop --pidfile $PIDFILE --oknodo
    rm -f $PIDFILE
    ;;
  restart)
    $0 stop
    sleep 1
    $0 start
    ;;
  *)
    echo "Usage: $0 {start|stop|restart}"
    exit 1
    ;;
esac
exit 0
`, binPath)

	scriptPath := "/etc/init.d/justping-agent"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return err
	}

	if _, err := exec.LookPath("update-rc.d"); err == nil {
		_ = exec.Command("update-rc.d", "justping-agent", "defaults").Run()
	} else if _, err := exec.LookPath("chkconfig"); err == nil {
		_ = exec.Command("chkconfig", "--add", "justping-agent").Run()
	}

	_ = exec.Command("/etc/init.d/justping-agent", "start").Run()
	fmt.Println("[Installer] Service installed and started via SysVinit!")
	return nil
}

func installWindows(binPath, serverURL, token string) error {
	appData := os.Getenv("ProgramData")
	if appData == "" {
		appData = "C:\\ProgramData"
	}
	cfgDir := filepath.Join(appData, "JustPing")
	_ = os.MkdirAll(cfgDir, 0755)
	cfgFile := filepath.Join(cfgDir, "agent.json")

	cfgJSON := fmt.Sprintf(`{
  "server": "%s",
  "token": "%s"
}
`, serverURL, token)
	_ = os.WriteFile(cfgFile, []byte(cfgJSON), 0644)

	binCmd := fmt.Sprintf("\"%s\" --config \"%s\"", binPath, cfgFile)
	_ = exec.Command("sc", "create", "JustPingAgent", "binPath=", binCmd, "start=", "auto").Run()
	_ = exec.Command("sc", "start", "JustPingAgent").Run()
	fmt.Println("[Installer] Windows Service JustPingAgent created and started.")
	return nil
}
