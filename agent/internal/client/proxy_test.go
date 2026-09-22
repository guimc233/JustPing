package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
