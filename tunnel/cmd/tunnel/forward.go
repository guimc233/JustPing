package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

// ForwardRule accepts TCP on a local port and relays every connection to a
// remote target through the probe. It is the "ssh -L"-shaped entry point, for
// clients that cannot be pointed at a proxy at all.
type ForwardRule struct {
	LocalAddr string
	Target    string

	listener  net.Listener
	done      chan struct{}
	closeOnce sync.Once
}

// Forwarder owns the port-forward rules of one session.
type Forwarder struct {
	dialer *Dialer
	logf   func(format string, args ...any)

	mu    sync.Mutex
	rules []*ForwardRule
}

// NewForwarder creates a forwarder that reports activity through logf.
func NewForwarder(d *Dialer, logf func(string, ...any)) *Forwarder {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Forwarder{dialer: d, logf: logf}
}

// Add parses a rule and starts listening. Two forms are accepted:
//
//	localPort:host:remotePort            binds 127.0.0.1
//	bindHost:localPort:host:remotePort   binds bindHost (loopback only)
func (f *Forwarder) Add(spec string) (*ForwardRule, error) {
	local, target, err := parseForwardSpec(spec)
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", local)
	if err != nil {
		return nil, err
	}

	rule := &ForwardRule{
		LocalAddr: ln.Addr().String(),
		Target:    target,
		listener:  ln,
		done:      make(chan struct{}),
	}

	f.mu.Lock()
	f.rules = append(f.rules, rule)
	f.mu.Unlock()

	f.logf("forward %s -> %s", rule.LocalAddr, target)
	go f.serve(rule)
	return rule, nil
}

// serve accepts connections on one rule until it is closed.
func (f *Forwarder) serve(rule *ForwardRule) {
	for {
		conn, err := rule.listener.Accept()
		if err != nil {
			select {
			case <-rule.done:
				// Closed on purpose.
			default:
				f.logf("forward %s stopped: %v", rule.LocalAddr, err)
			}
			return
		}
		go f.handle(rule, conn)
	}
}

func (f *Forwarder) handle(rule *ForwardRule, conn net.Conn) {
	defer conn.Close()

	upstream, _, err := f.dialer.Dial(rule.Target)
	if err != nil {
		f.logf("forward %s -> %s failed: %v", rule.LocalAddr, rule.Target, err)
		return
	}
	relay(conn, upstream)
}

// Rules returns a snapshot of the active rules.
func (f *Forwarder) Rules() []*ForwardRule {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*ForwardRule, len(f.rules))
	copy(out, f.rules)
	return out
}

// CloseAll stops every rule.
func (f *Forwarder) CloseAll() {
	for _, rule := range f.Rules() {
		rule.closeOnce.Do(func() {
			close(rule.done)
			_ = rule.listener.Close()
		})
	}
	f.mu.Lock()
	f.rules = nil
	f.mu.Unlock()
}

// parseForwardSpec splits a rule into a loopback-safe listen address and a
// remote target.
func parseForwardSpec(spec string) (string, string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", fmt.Errorf("empty forward rule")
	}
	fields := strings.Split(spec, ":")
	if len(fields) < 3 || len(fields) > 4 {
		return "", "", fmt.Errorf("invalid forward rule %q: want localPort:host:remotePort", spec)
	}

	var bindHost string
	var localPortField, targetHost, remotePort string
	if len(fields) == 3 {
		bindHost = "127.0.0.1"
		localPortField, targetHost, remotePort = fields[0], fields[1], fields[2]
	} else {
		bindHost, localPortField, targetHost, remotePort = fields[0], fields[1], fields[2], fields[3]
	}

	if err := ensureLoopback(bindHost); err != nil {
		return "", "", err
	}
	localPort, err := parsePort(localPortField)
	if err != nil {
		return "", "", fmt.Errorf("invalid local port in %q: %w", spec, err)
	}
	if targetHost == "" {
		return "", "", fmt.Errorf("invalid forward rule %q: missing target host", spec)
	}
	remotePortNum, err := parsePort(remotePort)
	if err != nil {
		return "", "", fmt.Errorf("invalid remote port in %q: %w", spec, err)
	}

	local := net.JoinHostPort(bindHost, strconv.Itoa(localPort))
	target := net.JoinHostPort(targetHost, strconv.Itoa(remotePortNum))
	return local, target, nil
}

func parsePort(field string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(field))
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", field)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%d is out of range", port)
	}
	return port, nil
}
