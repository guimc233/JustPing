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
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Version is injected at build time with -X main.Version=<tag>.
var Version = "dev"

func main() {
	var (
		serverFlag     = flag.String("server", "", "JustPing Host URL (e.g. https://ping.example.com)")
		credentialFlag = flag.String("credential", "", "Proxy credential as user:password (or set JUSTPING_PROXY_CREDENTIAL)")
		listenFlag     = flag.String("listen", "", "Also listen on this loopback address as an HTTP proxy (e.g. 127.0.0.1:8899)")
		insecureFlag   = flag.Bool("insecure", false, "Skip TLS verification of the Host certificate")
		versionFlag    = flag.Bool("version", false, "Show version and exit")
	)
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

	dialer, err := NewDialer(server, user, pass, *insecureFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	proxy := NewLocalProxy(dialer)
	m := newModel(server, user, dialer, proxy)

	// A non-empty --listen starts the local proxy immediately instead of waiting
	// for the toggle, so the client is usable from a script without the TUI.
	if *listenFlag != "" {
		if err := proxy.Start(*listenFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		m.log("local proxy listening on %s", proxy.Addr())
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		proxy.Stop()
		dialer.CloseAll()
		os.Exit(1)
	}

	_ = proxy.Stop()
	dialer.CloseAll()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
