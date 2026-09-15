package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/guimc233/JustPing/agent/internal/client"
	"github.com/guimc233/JustPing/agent/internal/pinger"
	"github.com/guimc233/JustPing/agent/internal/service"
	"github.com/guimc233/JustPing/shared/protocol"
)

var (
	Version   = "1.0.0"
	GitCommit = "unknown"
)

type FileConfig struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

func main() {
	serverFlag := flag.String("server", "", "JustPing Host URL (e.g. https://ping.example.com)")
	tokenFlag := flag.String("token", "", "Agent enrollment token")
	configFlag := flag.String("config", "", "Path to configuration file")
	installFlag := flag.Bool("install", false, "Install as OS service")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall OS service")
	versionFlag := flag.Bool("version", false, "Show version and build info")

	flag.Parse()

	if *versionFlag {
		fmt.Printf("JustPing Agent v%s (%s)\n", Version, GitCommit)
		return
	}

	if *uninstallFlag {
		if err := service.UninstallService(); err != nil {
			log.Fatalf("Uninstall failed: %v", err)
		}
		return
	}

	serverURL := *serverFlag
	token := *tokenFlag

	if serverURL == "" {
		serverURL = os.Getenv("JUSTPING_SERVER")
	}
	if token == "" {
		token = os.Getenv("JUSTPING_TOKEN")
	}

	cfgPath := *configFlag
	if cfgPath == "" {
		candidates := []string{"/etc/justping/agent.json", "agent.json"}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				cfgPath = c
				break
			}
		}
	}

	if cfgPath != "" {
		if data, err := os.ReadFile(cfgPath); err == nil {
			var fc FileConfig
			if err := json.Unmarshal(data, &fc); err == nil {
				if serverURL == "" {
					serverURL = fc.Server
				}
				if token == "" {
					token = fc.Token
				}
			}
		}
	}

	if *installFlag {
		if serverURL == "" || token == "" {
			log.Fatalf("Error: --server and --token are required to install service")
		}
		execPath, err := os.Executable()
		if err != nil {
			log.Fatalf("Failed to determine executable path: %v", err)
		}
		execPath, _ = filepath.EvalSymlinks(execPath)
		if err := service.InstallService(execPath, serverURL, token); err != nil {
			log.Fatalf("Installation failed: %v", err)
		}
		return
	}

	if serverURL == "" || token == "" {
		fmt.Println("Error: Server URL and Token are required.")
		fmt.Println("Usage: justping-agent --server <URL> --token <TOKEN>")
		fmt.Println("   or: justping-agent --install --server <URL> --token <TOKEN>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Printf("Starting JustPing Agent v%s...\n", Version)
	log.Printf("Target Host: %s\n", serverURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := pinger.NewPinger()
	var sched *TargetScheduler

	c := client.NewClient(
		client.Config{ServerURL: serverURL, Token: token, Version: Version},
		p,
		func(targets []protocol.TargetConfig) {
			if sched != nil {
				sched.SyncTargets(targets)
			}
		},
	)

	sched = NewTargetScheduler(ctx, p, c)
	c.Start(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down JustPing Agent...")
	sched.Stop()
	cancel()
	c.Stop()
	time.Sleep(300 * time.Millisecond)
	log.Println("Agent stopped.")
}
