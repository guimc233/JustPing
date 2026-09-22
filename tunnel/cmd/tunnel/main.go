// Command justping-tunnel is the interactive console for the Host's temporary
// HTTPS exit: it tunnels traffic through one JustPing probe and shows it.
//
// It connects over the WebSocket tunnel endpoint rather than a raw HTTP
// CONNECT, because a reverse proxy in front of the Host usually refuses the
// CONNECT method (nginx answers 405 Not Allowed) while forwarding WebSocket
// upgrades. That keeps everything on the Host's existing port and TLS listener.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

// Version is injected at build time with -X main.Version=<tag>.
var Version = "dev"

func main() {
	var (
		serverFlag     = flag.String("server", "", "JustPing Host URL (e.g. https://ping.example.com)")
		credentialFlag = flag.String("credential", "", "Proxy credential as user:password (or set JUSTPING_PROXY_CREDENTIAL)")
		listenFlag     = flag.String("listen", "", "Also listen on this loopback address as a SOCKS5 + HTTP proxy (e.g. 127.0.0.1:8899)")
		noTUIFlag      = flag.Bool("no-tui", false, "Run without the interactive console, keeping --listen and --forward in the foreground")
		forwardFlags   = multiFlag{}
		insecureFlag   = flag.Bool("insecure", false, "Skip TLS verification of the Host certificate")
		versionFlag    = flag.Bool("version", false, "Show version and exit")
	)
	flag.Var(&forwardFlags, "forward", "Forward a local port through the probe, as localPort:host:port (repeatable)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("JustPing Tunnel %s\n", Version)
		return
	}

	server := firstNonEmpty(*serverFlag, os.Getenv("JUSTPING_SERVER"))
	credential := firstNonEmpty(*credentialFlag, os.Getenv("JUSTPING_PROXY_CREDENTIAL"))

	if server == "" || credential == "" {
		fmt.Fprintln(os.Stderr, "Error: --server and --credential are required.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage: justping-tunnel --server <URL> --credential <user:password>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Issue a credential from the Probes table in the JustPing web UI: click the")
		fmt.Fprintln(os.Stderr, "globe icon, then copy the one-line launch command it shows.")
		flag.PrintDefaults()
		os.Exit(2)
	}

	user, pass, ok := strings.Cut(credential, ":")
	if !ok || user == "" || pass == "" {
		fmt.Fprintln(os.Stderr, "Error: --credential must be in user:password form.")
		os.Exit(2)
	}

	logs := newLogBuffer(200)
	logf := logs.add

	dialer, err := NewDialer(server, user, pass, *insecureFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	proxy := NewLocalProxy(dialer, logf)
	forwarder := NewForwarder(dialer, logf)
	cleanup := func() {
		_ = proxy.Stop()
		forwarder.CloseAll()
		dialer.CloseAll()
	}

	if *listenFlag != "" {
		if err := proxy.Start(*listenFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		logf("mixed proxy listening on %s", proxy.Addr())
	}
	for _, spec := range forwardFlags {
		if _, err := forwarder.Add(spec); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			cleanup()
			os.Exit(1)
		}
	}

	// Headless mode makes the client usable from scripts and services, where an
	// interactive console would just get in the way.
	if *noTUIFlag {
		runHeadless(proxy, forwarder)
		cleanup()
		return
	}

	m := newModel(server, user, dialer, proxy, forwarder, logs)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	cleanup()
}

// runHeadless blocks until the process is asked to stop, leaving the configured
// listeners running.
func runHeadless(proxy *LocalProxy, forwarder *Forwarder) {
	if proxy.Addr() != "" {
		fmt.Fprintf(os.Stderr, "mixed proxy on %s\n", proxy.Addr())
	}
	for _, rule := range forwarder.Rules() {
		fmt.Fprintf(os.Stderr, "forward %s -> %s\n", rule.LocalAddr, rule.Target)
	}
	if proxy.Addr() == "" && len(forwarder.Rules()) == 0 {
		fmt.Fprintln(os.Stderr, "nothing to do: pass --listen and/or --forward with --no-tui")
		return
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
}

// multiFlag collects a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
