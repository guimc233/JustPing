package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// inputMode says what the input line is currently collecting.
type inputMode int

const (
	modeNone inputMode = iota
	modeTarget
	modeRequest
	modeListen
)

type tickMsg time.Time
type logMsg string

type targetResultMsg struct {
	addr string
	err  error
}

type requestResultMsg struct {
	url      string
	status   int
	duration time.Duration
	headers  []string
	body     string
	err      error
}

type listenResultMsg struct {
	addr string
	err  error
}

// model is the TUI state.
type model struct {
	dialer *Dialer
	proxy  *LocalProxy

	server string
	user   string

	mode   inputMode
	input  string
	prompt string

	logs   []string
	logMax int

	reqs   []requestResultMsg
	reqMax int

	width  int
	height int
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	labelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	valueStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	headStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
)

func newModel(server, user string, d *Dialer, p *LocalProxy) model {
	return model{
		dialer: d,
		proxy:  p,
		server: server,
		user:   user,
		logs:   []string{},
		logMax: 200,
		reqs:   []requestResultMsg{},
		reqMax: 20,
	}
}

func (m model) Init() tea.Cmd {
	// The alt screen is entered by tea.WithAltScreen; only the refresh ticker is
	// needed here.
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) log(format string, args ...any) {
	line := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
	m.logs = append(m.logs, line)
	if len(m.logs) > m.logMax {
		m.logs = m.logs[len(m.logs)-m.logMax:]
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		return m, tick()

	case logMsg:
		m.log("%s", string(msg))
		return m, nil

	case targetResultMsg:
		if msg.err != nil {
			m.log("open %s failed: %v", msg.addr, msg.err)
		} else {
			m.log("tunnel open to %s", msg.addr)
		}
		return m, nil

	case listenResultMsg:
		if msg.err != nil {
			m.log("local proxy failed: %v", msg.err)
		} else if msg.addr == "" {
			m.log("local proxy stopped")
		} else {
			m.log("local proxy listening on %s (export https_proxy=http://%s)", msg.addr, msg.addr)
		}
		return m, nil

	case requestResultMsg:
		m.reqs = append(m.reqs, msg)
		if len(m.reqs) > m.reqMax {
			m.reqs = m.reqs[len(m.reqs)-m.reqMax:]
		}
		if msg.err != nil {
			m.log("request failed: %v", msg.err)
		} else {
			m.log("%s -> %d in %s", msg.url, msg.status, msg.duration.Round(time.Millisecond))
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeNone {
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.mode = modeNone
			m.input = ""
			return m, nil
		case tea.KeyEnter:
			value := strings.TrimSpace(m.input)
			// Capture the mode before clearing it: submit dispatches on which
			// prompt was active, and clearing it first made every action a no-op.
			mode := m.mode
			m.mode = modeNone
			m.input = ""
			if value == "" {
				return m, nil
			}
			return m.submit(mode, value)
		case tea.KeyBackspace:
			if len(m.input) > 0 {
				r := []rune(m.input)
				m.input = string(r[:len(r)-1])
			}
			return m, nil
		case tea.KeySpace:
			m.input += " "
			return m, nil
		case tea.KeyRunes:
			m.input += string(msg.Runes)
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "t":
		m.mode = modeTarget
		m.prompt = "target host:port"
		m.input = ""
	case "r":
		m.mode = modeRequest
		m.prompt = "request URL"
		m.input = "https://"
	case "l":
		return m.toggleLocalProxy()
	case "x":
		n := len(m.dialer.Tunnels())
		m.dialer.CloseAll()
		m.log("closed %d tunnel(s)", n)
	case "c":
		m.logs = []string{}
		m.reqs = []requestResultMsg{}
	}
	return m, nil
}

func (m model) submit(mode inputMode, value string) (tea.Model, tea.Cmd) {
	switch mode {
	case modeListen:
		m.log("starting local proxy on %s", value)
		return m, startListen(m.proxy, value)
	case modeTarget:
		m.log("opening tunnel to %s", value)
		return m, openTunnel(m.dialer, value)
	case modeRequest:
		m.log("requesting %s", value)
		return m, doRequest(m.dialer, value)
	}
	return m, nil
}

func (m model) toggleLocalProxy() (tea.Model, tea.Cmd) {
	if m.proxy.Running() {
		if err := m.proxy.Stop(); err != nil {
			m.log("stop failed: %v", err)
			return m, nil
		}
		return m, func() tea.Msg { return listenResultMsg{} }
	}
	m.mode = modeListen
	m.prompt = "listen address"
	m.input = "127.0.0.1:8899"
	return m, nil
}

// Commands.

func openTunnel(d *Dialer, addr string) tea.Cmd {
	return func() tea.Msg {
		conn, _, err := d.Dial(addr)
		if err == nil {
			// Keep the idle tunnel open so it shows up in the list with its byte
			// counters; it is released on exit.
			_ = conn
		}
		return targetResultMsg{addr: addr, err: err}
	}
}

func startListen(p *LocalProxy, addr string) tea.Cmd {
	return func() tea.Msg {
		if err := p.Start(addr); err != nil {
			return listenResultMsg{err: err}
		}
		return listenResultMsg{addr: p.Addr()}
	}
}

func doRequest(d *Dialer, rawURL string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return requestResultMsg{url: rawURL, err: err}
		}
		req.Header.Set("User-Agent", "justping-tunnel/"+Version)

		start := time.Now()
		resp, err := d.HTTPClient().Do(req)
		elapsed := time.Since(start)
		if err != nil {
			return requestResultMsg{url: rawURL, err: err, duration: elapsed}
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		if err != nil {
			return requestResultMsg{url: rawURL, err: err, duration: elapsed}
		}

		var headers []string
		for _, k := range []string{"Server", "Content-Type", "Content-Length"} {
			if v := resp.Header.Get(k); v != "" {
				headers = append(headers, k+": "+v)
			}
		}
		return requestResultMsg{
			url:      rawURL,
			status:   resp.StatusCode,
			duration: elapsed,
			headers:  headers,
			body:     strings.TrimSpace(string(body)),
		}
	}
}

// View.

func (m model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("JustPing Tunnel") + "\n")
	b.WriteString(borderStyle.Render(strings.Repeat("─", m.frameWidth())) + "\n")

	b.WriteString(m.statusBlock())
	b.WriteString("\n")
	b.WriteString(headStyle.Render("Tunnels") + "\n")
	b.WriteString(m.tunnelsBlock())
	b.WriteString("\n")
	b.WriteString(headStyle.Render("Requests") + "\n")
	b.WriteString(m.requestsBlock())
	b.WriteString("\n")
	b.WriteString(headStyle.Render("Log") + "\n")
	b.WriteString(m.logBlock())
	b.WriteString("\n")

	if m.mode != modeNone {
		b.WriteString(warnStyle.Render("> "+m.prompt+": ") + m.input + "▏\n")
		b.WriteString(helpStyle.Render("enter confirm · esc cancel") + "\n")
	} else {
		b.WriteString(helpStyle.Render("[t] open tunnel  [r] request  [l] local proxy  [x] close tunnels  [c] clear  [q] quit") + "\n")
	}

	return b.String()
}

func (m model) frameWidth() int {
	if m.width > 0 && m.width < 120 {
		return m.width
	}
	return 100
}

func (m model) statusBlock() string {
	rows := [][2]string{
		{"Server", m.server},
		{"Endpoint", m.dialer.URL()},
		{"Credential", m.user + " (10 min TTL, re-issue when it expires)"},
	}
	if m.proxy.Running() {
		rows = append(rows, [2]string{"Local proxy", okStyle.Render("ON") + "  " + m.proxy.Addr()})
	} else {
		rows = append(rows, [2]string{"Local proxy", labelStyle.Render("OFF")})
	}

	var b strings.Builder
	for _, r := range rows {
		b.WriteString(" " + labelStyle.Render(pad(r[0], 12)) + " " + valueStyle.Render(r[1]) + "\n")
	}
	return b.String()
}

func (m model) tunnelsBlock() string {
	tunnels := m.dialer.Tunnels()
	if len(tunnels) == 0 {
		return " " + labelStyle.Render("none") + "\n"
	}
	var b strings.Builder
	b.WriteString(" " + labelStyle.Render(padRight("#", 3)+padRight("target", 34)+padRight("tx", 10)+padRight("rx", 10)) + "status\n")
	for _, t := range tunnels {
		tx, rx := t.Bytes()
		closed, err := t.Done()
		status := okStyle.Render("open")
		if closed {
			status = labelStyle.Render("closed")
		}
		if err != nil {
			status = errStyle.Render("error")
		}
		b.WriteString(" " + fmt.Sprintf("%s%s%s%s%s\n",
			padRight(fmt.Sprint(t.ID), 3),
			padRight(t.Target, 34),
			padRight(humanBytes(tx), 10),
			padRight(humanBytes(rx), 10),
			status,
		))
	}
	return b.String()
}

func (m model) requestsBlock() string {
	if len(m.reqs) == 0 {
		return " " + labelStyle.Render("none") + "\n"
	}
	var b strings.Builder
	last := m.reqs[len(m.reqs)-1]
	if last.err != nil {
		b.WriteString(" " + errStyle.Render(last.url+": "+last.err.Error()) + "\n")
		return b.String()
	}
	statusStyle := okStyle
	if last.status >= 400 {
		statusStyle = errStyle
	}
	b.WriteString(" " + valueStyle.Render(last.url) + "  " + statusStyle.Render(fmt.Sprint(last.status)) +
		"  " + labelStyle.Render(last.duration.Round(time.Millisecond).String()) + "\n")
	for _, h := range last.headers {
		b.WriteString(" " + labelStyle.Render(h) + "\n")
	}
	if last.body != "" {
		for _, line := range strings.Split(truncateLines(last.body, 12), "\n") {
			b.WriteString(" " + line + "\n")
		}
	}
	return b.String()
}

func (m model) logBlock() string {
	if len(m.logs) == 0 {
		return " " + labelStyle.Render("no activity yet") + "\n"
	}
	available := 6
	if m.height > 30 {
		available = m.height - 30
		if available < 4 {
			available = 4
		}
	}
	logs := m.logs
	if len(logs) > available {
		logs = logs[len(logs)-available:]
	}
	var b strings.Builder
	for _, l := range logs {
		b.WriteString(" " + l + "\n")
	}
	return b.String()
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func truncateLines(s string, max int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= max {
		return s
	}
	return strings.Join(lines[:max], "\n") + "\n…"
}
