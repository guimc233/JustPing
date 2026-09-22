package main

import (
	"context"
	"fmt"
	"io"
	"log"
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

// LocalProxy is the optional 127.0.0.1 listener that makes the probe usable from
// ordinary tools such as curl or the browser.
//
// It binds the loopback interface only: the probe credential is short-lived and
// meant for the operator running this client, so it must not be exposed on the
// network. It stays off until the operator toggles it.
type LocalProxy struct {
	dialer *Dialer

	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
	addr     string
}

// NewLocalProxy creates a stopped local proxy.
func NewLocalProxy(d *Dialer) *LocalProxy {
	return &LocalProxy{dialer: d}
}

// Addr reports the listen address, or "" when stopped.
func (p *LocalProxy) Addr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addr
}

// Running reports whether the listener is active.
func (p *LocalProxy) Running() bool { return p.Addr() != "" }

// Start binds addr (e.g. "127.0.0.1:8899") and begins serving.
func (p *LocalProxy) Start(addr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.listener != nil {
		return fmt.Errorf("already listening on %s", p.addr)
	}
	if addr == "" {
		addr = "127.0.0.1:8899"
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	// Refuse anything but loopback: this listener is not authenticated.
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("refusing to listen on %s: only loopback addresses are allowed", host)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Handler:           p,
		ReadHeaderTimeout: 10 * time.Second,
	}
	p.listener = ln
	p.server = srv
	p.addr = ln.Addr().String()

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[proxy] listener stopped: %v", err)
		}
		p.mu.Lock()
		p.listener = nil
		p.server = nil
		p.addr = ""
		p.mu.Unlock()
	}()

	return nil
}

// Stop closes the listener.
func (p *LocalProxy) Stop() error {
	p.mu.Lock()
	srv := p.server
	p.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
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
