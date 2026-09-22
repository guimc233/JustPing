package proxy

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/guimc233/JustPing/shared/protocol"
)

type exitBridge struct {
	svc   *Service
	mu    sync.Mutex
	conns map[string]net.Conn
}

func (b *exitBridge) Online(string) bool { return true }

func (b *exitBridge) Send(agentID string, msgType protocol.MessageType, payload any) error {
	switch msgType {
	case protocol.TypeProxyOpen:
		var req protocol.ProxyOpenPayload
		if err := remarshal(payload, &req); err != nil {
			return err
		}
		go b.dial(agentID, req)
	case protocol.TypeProxyData:
		var msg protocol.ProxyDataPayload
		if err := remarshal(payload, &msg); err != nil {
			return err
		}
		raw, err := base64.StdEncoding.DecodeString(msg.Data)
		if err != nil {
			return err
		}
		b.mu.Lock()
		conn := b.conns[msg.TunnelID]
		b.mu.Unlock()
		if conn == nil {
			return io.ErrClosedPipe
		}
		_, err = conn.Write(raw)
		return err
	case protocol.TypeProxyClose:
		var msg protocol.ProxyClosePayload
		if err := remarshal(payload, &msg); err != nil {
			return err
		}
		b.mu.Lock()
		conn := b.conns[msg.TunnelID]
		delete(b.conns, msg.TunnelID)
		b.mu.Unlock()
		if conn != nil {
			_ = conn.Close()
		}
	}
	return nil
}

func (b *exitBridge) dial(agentID string, req protocol.ProxyOpenPayload) {
	conn, err := net.Dial("tcp", net.JoinHostPort(req.Host, strconv.Itoa(req.Port)))
	if err != nil {
		b.svc.HandleAgent(agentID, protocol.Envelope{Type: protocol.TypeProxyOpenResult, Payload: protocol.ProxyOpenResultPayload{
			TunnelID: req.TunnelID, OK: false, Error: err.Error(),
		}})
		return
	}
	b.mu.Lock()
	if b.conns == nil {
		b.conns = make(map[string]net.Conn)
	}
	b.conns[req.TunnelID] = conn
	b.mu.Unlock()
	b.svc.HandleAgent(agentID, protocol.Envelope{Type: protocol.TypeProxyOpenResult, Payload: protocol.ProxyOpenResultPayload{
		TunnelID: req.TunnelID, OK: true,
	}})
	buf := make([]byte, 1024)
	for {
		n, readErr := conn.Read(buf)
		if n > 0 {
			b.svc.HandleAgent(agentID, protocol.Envelope{Type: protocol.TypeProxyData, Payload: protocol.ProxyDataPayload{
				TunnelID: req.TunnelID,
				Data:     base64.StdEncoding.EncodeToString(buf[:n]),
			}})
		}
		if readErr != nil {
			b.svc.HandleAgent(agentID, protocol.Envelope{Type: protocol.TypeProxyClose, Payload: protocol.ProxyClosePayload{TunnelID: req.TunnelID}})
			return
		}
	}
}

func remarshal(payload any, dest any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func TestConnectRelaysThroughProbe(t *testing.T) {
	target, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	go func() {
		conn, err := target.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		_, _ = conn.Write([]byte("pong"))
	}()

	svc := NewService()
	bridge := &exitBridge{svc: svc, conns: make(map[string]net.Conn)}
	svc.SetBridge(bridge)
	issued, err := svc.Sessions().Issue("agent-1", "lab", 1)
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(svc.HandleConnect))
	defer ts.Close()

	conn, err := net.Dial("tcp", ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	token := base64.StdEncoding.EncodeToString([]byte(issued.Username + ":" + issued.Password))
	targetAddr := target.Addr().String()
	if _, err := conn.Write([]byte("CONNECT " + targetAddr + " HTTP/1.1\r\nHost: " + targetAddr + "\r\nProxy-Authorization: Basic " + token + "\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 4)
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "pong" {
		t.Fatalf("relay = %q", got)
	}
}

func TestConnectRequiresCredential(t *testing.T) {
	svc := NewService()
	svc.SetBridge(&exitBridge{svc: svc})
	ts := httptest.NewServer(http.HandlerFunc(svc.HandleConnect))
	defer ts.Close()

	conn, err := net.Dial("tcp", ts.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if resp.Header.Get("Proxy-Authenticate") == "" {
		t.Fatal("missing Proxy-Authenticate")
	}
}
