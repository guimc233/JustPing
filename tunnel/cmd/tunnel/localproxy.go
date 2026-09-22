package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// DialContext lets an http.Transport route every connection through the Host and
// the selected probe, which is what powers the request console.
func (d *Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("unsupported network %q", network)
	}

	ch := make(chan dialResult, 1)
	go func() {
		conn, _, err := d.Dial(addr)
		ch <- dialResult{conn: conn, err: err}
	}()

	select {
	case <-ctx.Done():
		// The dial cannot be interrupted, so release the tunnel whenever it
		// eventually arrives rather than leaking it.
		go func() {
			if r := <-ch; r.conn != nil {
				_ = r.conn.Close()
			}
		}()
		return nil, ctx.Err()
	case r := <-ch:
		return r.conn, r.err
	}
}

type dialResult struct {
	conn net.Conn
	err  error
}

// clientTimeout bounds one request end to end.
const clientTimeout = 60 * time.Second

// HTTPClient returns the shared client whose traffic exits through the probe.
//
// One client is reused for the whole run so idle connections are pooled. That
// matters because every connection opens a tunnel on the Host, and the Host caps
// concurrent tunnels per credential: a fresh transport per request would leak an
// idle tunnel every time and eventually hit that cap.
func (d *Dialer) HTTPClient() *http.Client {
	d.clientOnce.Do(func() {
		d.client = &http.Client{
			Transport: &http.Transport{
				DialContext:           d.DialContext,
				MaxIdleConns:          8,
				MaxIdleConnsPerHost:   2,
				IdleConnTimeout:       30 * time.Second,
				TLSHandshakeTimeout:   15 * time.Second,
				ResponseHeaderTimeout: clientTimeout,
			},
			Timeout: clientTimeout,
		}
	})
	return d.client
}

// LocalProxy is the optional loopback listener that makes the probe usable from
// ordinary tools: curl, a browser, or anything that speaks SOCKS5.
//
// One port serves both, chosen by peeking the first byte: SOCKS5 always begins
// with 0x05, while an HTTP request begins with a method letter. Sharing the port
// keeps the client to a single local endpoint regardless of how the tool expects
// to be configured.
//
// It binds the loopback interface only: the probe credential is short-lived and
// meant for the operator running this client, so it must not be exposed on the
// network. It stays off until the operator toggles it.
type LocalProxy struct {
	dialer *Dialer
	logf   func(format string, args ...any)

	mu       sync.Mutex
	listener net.Listener
	addr     string
	closed   chan struct{}
}

// NewLocalProxy creates a stopped local proxy.
func NewLocalProxy(d *Dialer, logf func(string, ...any)) *LocalProxy {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &LocalProxy{dialer: d, logf: logf}
}

// Addr reports the listen address, or "" when stopped.
func (p *LocalProxy) Addr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addr
}

// Running reports whether the listener is active.
func (p *LocalProxy) Running() bool { return p.Addr() != "" }

// ensureLoopback rejects any bind address that is not loopback. It guards both
// the proxy listener and the forward rules, neither of which authenticates
// clients.
func ensureLoopback(host string) error {
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("refusing to listen on %s: only loopback addresses are allowed", host)
	}
	return nil
}

// Start binds addr (e.g. "127.0.0.1:8899") and begins serving.
func (p *LocalProxy) Start(addr string) error {
	p.mu.Lock()
	if p.listener != nil {
		p.mu.Unlock()
		return fmt.Errorf("already listening on %s", p.addr)
	}
	if addr == "" {
		addr = "127.0.0.1:8899"
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	if err := ensureLoopback(host); err != nil {
		p.mu.Unlock()
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		p.mu.Unlock()
		return err
	}

	p.listener = ln
	p.addr = ln.Addr().String()
	p.closed = make(chan struct{})
	closed := p.closed
	p.mu.Unlock()

	go p.serve(ln, closed)
	return nil
}

// serve accepts connections and dispatches each to SOCKS5 or HTTP.
func (p *LocalProxy) serve(ln net.Listener, closed chan struct{}) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-closed:
				// Stopped on purpose.
			default:
				p.logf("local proxy stopped: %v", err)
			}
			return
		}
		go p.serveConn(conn)
	}
}

// serveConn peeks at the first byte to decide which protocol the client speaks.
func (p *LocalProxy) serveConn(conn net.Conn) {
	br := bufio.NewReader(conn)
	first, err := br.Peek(1)
	if err != nil {
		_ = conn.Close()
		return
	}
	if first[0] == socksVersion {
		p.handleSOCKS5(conn, br)
		return
	}
	p.serveHTTP(conn, br)
}

// Stop closes the listener.
func (p *LocalProxy) Stop() error {
	p.mu.Lock()
	ln := p.listener
	closed := p.closed
	p.listener = nil
	p.addr = ""
	p.closed = nil
	p.mu.Unlock()

	if ln == nil {
		return nil
	}
	if closed != nil {
		close(closed)
	}
	return ln.Close()
}

// peekConn serves reads from a buffer that already holds bytes consumed while
// sniffing the protocol.
type peekConn struct {
	net.Conn
	r io.Reader
}

func (c *peekConn) Read(b []byte) (int, error) { return c.r.Read(b) }

// singleConnListener hands one connection to http.Serve and then reports the
// listener as closed, which is the supported way to serve a single connection.
type singleConnListener struct {
	conn net.Conn
	once sync.Once
	done bool
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	var conn net.Conn
	l.once.Do(func() { conn = l.conn })
	if conn != nil {
		return conn, nil
	}
	return nil, errors.New("listener closed")
}

func (l *singleConnListener) Close() error { return nil }

func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }

// serveHTTP runs the HTTP proxy handler over one already-accepted connection.
func (p *LocalProxy) serveHTTP(conn net.Conn, br *bufio.Reader) {
	srv := &http.Server{
		Handler:           p,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// Errors are expected once the listener reports itself closed.
	_ = srv.Serve(&singleConnListener{conn: &peekConn{Conn: conn, r: br}})
}

// ServeHTTP implements an HTTP proxy for both CONNECT and absolute-form requests.
func (p *LocalProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	if r.URL == nil || !r.URL.IsAbs() {
		http.Error(w, "this is a proxy; send absolute-form requests or CONNECT", http.StatusBadRequest)
		return
	}
	p.handleForward(w, r)
}

func (p *LocalProxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	target := r.Host
	if r.URL != nil && r.URL.Host != "" {
		target = r.URL.Host
	}

	upstream, _, err := p.dialer.Dial(target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	client, buf, err := hj.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	if _, err := client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		_ = upstream.Close()
		_ = client.Close()
		return
	}

	// Bytes already buffered by the server must be replayed first.
	if buf != nil && buf.Reader != nil && buf.Reader.Buffered() > 0 {
		_, _ = io.Copy(upstream, io.LimitReader(buf.Reader, int64(buf.Reader.Buffered())))
	}

	relay(client, upstream)
}

func (p *LocalProxy) handleForward(w http.ResponseWriter, r *http.Request) {
	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	outReq.Header = r.Header.Clone()
	// The client's proxy credentials are for us, not for the target.
	outReq.Header.Del("Proxy-Authorization")
	outReq.Header.Del("Proxy-Connection")

	client := p.dialer.HTTPClient()
	resp, err := client.Do(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// relay copies bytes both ways until either side closes.
func relay(a, b net.Conn) {
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		if c, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = c.CloseWrite()
		}
		done <- struct{}{}
	}
	go cp(a, b)
	go cp(b, a)
	<-done
	<-done
	_ = a.Close()
	_ = b.Close()
}
