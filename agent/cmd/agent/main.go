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
	"github.com/guimc233/JustPing/agent/internal/updater"
	"github.com/guimc233/JustPing/shared/protocol"
)

var (
	Version   = "1.0.11"
	GitCommit = "unknown"
)

type FileConfig = service.AgentConfig

func main() {
	serverFlag := flag.String("server", "", "JustPing Host URL (e.g. https://ping.example.com)")
	tokenFlag := flag.String("token", "", "Agent enrollment token")
	configFlag := flag.String("config", "", "Path to configuration file")
	chinaMirrorFlag := flag.Bool("china-mirror", false, "Use China mirror for downloads and updates")
	autoUpdateFlag := flag.Bool("auto-update", true, "Enable automatic updates (default: true)")
	updateFlag := flag.Bool("update", false, "Check and perform self-update immediately")
	forceFlag := flag.Bool("force", false, "Force update even if version matches")
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

	explicitFlags := make(map[string]bool)
	flag.CommandLine.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	var cfg service.AgentConfig

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
			_ = json.Unmarshal(data, &cfg)
		}
	}

	if envServer := os.Getenv("JUSTPING_SERVER"); envServer != "" && cfg.Server == "" {
		cfg.Server = envServer
	}
	if envToken := os.Getenv("JUSTPING_TOKEN"); envToken != "" && cfg.Token == "" {
		cfg.Token = envToken
	}
	if envMirror := os.Getenv("JUSTPING_CHINA_MIRROR"); envMirror == "1" || envMirror == "true" {
		cfg.ChinaMirror = true
	}

	if explicitFlags["server"] {
		cfg.Server = *serverFlag
	}
	if explicitFlags["token"] {
		cfg.Token = *tokenFlag
	}
	if explicitFlags["china-mirror"] {
		cfg.ChinaMirror = *chinaMirrorFlag
	}
	if explicitFlags["auto-update"] {
		val := *autoUpdateFlag
		cfg.AutoUpdate = &val
	}

	if *updateFlag {
		updaterCfg := updater.Config{
			CurrentVersion: Version,
			ChinaMirror:    cfg.ChinaMirror,
			Repo:           "guimc233/JustPing",
		}
		updated, ver, err := updater.RunUpdate(updaterCfg, *forceFlag)
		if err != nil {
			log.Fatalf("Update failed: %v", err)
		}
		if updated {
			fmt.Printf("Successfully updated JustPing Agent to %s\n", ver)
			updater.RestartActiveService()
		} else {
			fmt.Printf("JustPing Agent is already up to date (%s)\n", ver)
		}
		return
	}

	if *installFlag {
		if cfg.Server == "" || cfg.Token == "" {
			log.Fatalf("Error: --server and --token are required to install service")
		}
		execPath, err := os.Executable()
		if err != nil {
			log.Fatalf("Failed to determine executable path: %v", err)
		}
		execPath, _ = filepath.EvalSymlinks(execPath)
		if err := service.InstallService(execPath, cfg); err != nil {
			log.Fatalf("Installation failed: %v", err)
		}
		return
	}

	if cfg.Server == "" || cfg.Token == "" {
		fmt.Println("Error: Server URL and Token are required.")
		fmt.Println("Usage: justping-agent --server <URL> --token <TOKEN>")
		fmt.Println("   or: justping-agent --install --server <URL> --token <TOKEN>")
		fmt.Println("   or: justping-agent --update")
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Printf("Starting JustPing Agent v%s...\n", Version)
	log.Printf("Target Host: %s\n", cfg.Server)
	if cfg.ChinaMirror {
		log.Printf("China mirror: enabled\n")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := pinger.NewPinger()
	var sched *TargetScheduler

	c := client.NewClient(
		client.Config{ServerURL: cfg.Server, Token: cfg.Token, Version: Version},
		p,
		func(targets []protocol.TargetConfig) {
			if sched != nil {
				sched.SyncTargets(targets)
			}
		},
	)

	sched = NewTargetScheduler(ctx, p, c)
	c.Start(ctx)

	isUpdating := false
	if cfg.IsAutoUpdateEnabled() {
		updaterCfg := updater.Config{
			CurrentVersion: Version,
			ChinaMirror:    cfg.ChinaMirror,
			AutoUpdate:     true,
			Repo:           "guimc233/JustPing",
		}
		updater.StartAutoUpdate(ctx, updaterCfg, func(newVersion string) {
			isUpdating = true
			log.Printf("[Updater] Agent updated to %s. Restarting...", newVersion)
			cancel()
		})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Println("Shutting down JustPing Agent...")
	case <-ctx.Done():
		if isUpdating {
			log.Println("Restarting JustPing Agent following update...")
		}
	}

	sched.Stop()
	cancel()
	c.Stop()
	time.Sleep(300 * time.Millisecond)

	if isUpdating {
		execPath, err := os.Executable()
		if err == nil {
			execPath, _ = filepath.EvalSymlinks(execPath)
			_ = syscall.Exec(execPath, os.Args, os.Environ())
		}
		os.Exit(0)
	}

	log.Println("Agent stopped.")
}
