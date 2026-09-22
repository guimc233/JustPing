package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/guimc233/JustPing/shared/protocol"
)

func TestProxyManagerRelaysBytes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port := 0
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}

	var mu sync.Mutex
	var messages []protocol.Envelope
	m := &proxyManager{}
	m.init(func(msgType protocol.MessageType, payload any) error {
		mu.Lock()
		messages = append(messages, protocol.Envelope{Type: msgType, Payload: payload})
		mu.Unlock()
		return nil
	})
	m.dial = func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}
	m.open(protocol.ProxyOpenPayload{TunnelID: "tun", Host: host, Port: port})

	if !waitFor(t, &mu, &messages, protocol.TypeProxyOpenResult) {
		t.Fatal("probe did not accept tunnel")
	}

	remote := <-accepted
	defer remote.Close()
	m.write("tun", []byte("ping"))
	buf := make([]byte, 4)
	if _, err := io.ReadFull(remote, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "ping" {
		t.Fatalf("target got %q", buf)
	}
	if _, err := remote.Write([]byte("pong")); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, &mu, &messages, protocol.TypeProxyData) {
		t.Fatal("probe did not return target bytes")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, msg := range messages {
		if msg.Type != protocol.TypeProxyData {
			continue
		}
		raw, _ := json.Marshal(msg.Payload)
		var data protocol.ProxyDataPayload
		_ = json.Unmarshal(raw, &data)
		decoded, _ := base64.StdEncoding.DecodeString(data.Data)
		if string(decoded) != "pong" {
			t.Fatalf("host got %q", decoded)
		}
		return
	}
}

func waitFor(t *testing.T, mu *sync.Mutex, messages *[]protocol.Envelope, kind protocol.MessageType) bool {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		for _, msg := range *messages {
			if msg.Type == kind {
				mu.Unlock()
				return true
			}
		}
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func TestProxyManagerRejectsDuplicateTunnel(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	entered := make(chan struct{}, 1)
	var mu sync.Mutex
	var messages []protocol.Envelope
	m := &proxyManager{}
	m.init(func(msgType protocol.MessageType, payload any) error {
		mu.Lock()
		messages = append(messages, protocol.Envelope{Type: msgType, Payload: payload})
		mu.Unlock()
		return nil
	})
	m.dial = func(ctx context.Context, addr string) (net.Conn, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, errors.New("stopped")
	}

	m.open(protocol.ProxyOpenPayload{TunnelID: "tun", Host: "example.com", Port: 80})
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("dial did not start")
	}
	m.open(protocol.ProxyOpenPayload{TunnelID: "tun", Host: "example.com", Port: 81})
	if !waitFor(t, &mu, &messages, protocol.TypeProxyOpenResult) {
		t.Fatal("duplicate was not rejected")
	}

	mu.Lock()
	raw, err := json.Marshal(messages[0].Payload)
	mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	var res protocol.ProxyOpenResultPayload
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatal(err)
	}
	if res.OK || res.Error != "tunnel already open" {
		t.Fatalf("duplicate result: %#v", res)
	}
	m.mu.Lock()
	_, ok := m.tabs["tun"]
	m.mu.Unlock()
	if !ok {
		t.Fatal("duplicate open removed the active tunnel")
	}
}

func TestDropIgnoresStaleTunnel(t *testing.T) {
	m := &proxyManager{}
	m.init(func(protocol.MessageType, any) error { return nil })
	active := &proxyTunnel{writes: make(chan []byte, 1), cancel: func() {}}
	stale := &proxyTunnel{writes: make(chan []byte, 1), cancel: func() {}}
	m.mu.Lock()
	m.tabs["tun"] = active
	m.mu.Unlock()

	m.dropIf("tun", stale, true)
	m.mu.Lock()
	got := m.tabs["tun"]
	m.mu.Unlock()
	if got != active {
		t.Fatal("stale drop removed the active tunnel")
	}
}

func TestProxyWriteDoesNotBlockReceivePath(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	var mu sync.Mutex
	var messages []protocol.Envelope
	m := &proxyManager{}
	m.init(func(msgType protocol.MessageType, payload any) error {
		mu.Lock()
		messages = append(messages, protocol.Envelope{Type: msgType, Payload: payload})
		mu.Unlock()
		return nil
	})
	m.dial = func(context.Context, string) (net.Conn, error) {
		return clientConn, nil
	}
	m.open(protocol.ProxyOpenPayload{TunnelID: "tun", Host: "example.com", Port: 443})
	if !waitFor(t, &mu, &messages, protocol.TypeProxyOpenResult) {
		t.Fatal("probe did not accept tunnel")
	}

	finished := make(chan struct{})
	go func() {
		for range proxyWriteQueue + 8 {
			m.write("tun", []byte("x"))
		}
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("proxy write blocked")
	}
	if !waitFor(t, &mu, &messages, protocol.TypeProxyClose) {
		t.Fatal("expected tunnel drop when the write queue is full")
	}
}
