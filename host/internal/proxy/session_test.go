package proxy

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSessionIssueAuthenticateExpiryAndRevoke(t *testing.T) {
	store := NewStore()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	issued, err := store.Issue("agent-1", "Tokyo", 7)
	if err != nil {
		t.Fatal(err)
	}
	if issued.Password == "" || !strings.HasPrefix(issued.Username, "jp") {
		t.Fatalf("unexpected credential: %#v", issued)
	}
	if issued.ExpiresAt.Sub(issued.CreatedAt) != SessionTTL {
		t.Fatalf("ttl = %s", issued.ExpiresAt.Sub(issued.CreatedAt))
	}

	got, err := store.Authenticate(issued.Username, issued.Password)
	if err != nil || got.AgentID != "agent-1" {
		t.Fatalf("authenticate: %v %#v", err, got)
	}
	if _, err := store.Authenticate(issued.Username, "wrong-password"); err != ErrUnauthorized {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := store.Authenticate("nobody", issued.Password); err != ErrUnauthorized {
		t.Fatalf("unknown user: %v", err)
	}

	now = now.Add(SessionTTL)
	if _, err := store.Authenticate(issued.Username, issued.Password); err != ErrUnauthorized {
		t.Fatalf("expired credential accepted: %v", err)
	}
	if store.Revoke(issued.ID) {
		t.Fatal("expired credential should not revoke")
	}

	now = time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	store = NewStore()
	store.now = func() time.Time { return now }
	issued, err = store.Issue("agent-1", "Tokyo", 7)
	if err != nil {
		t.Fatal(err)
	}
	if !store.Revoke(issued.ID) {
		t.Fatal("expected revoke")
	}
	if _, err := store.Authenticate(issued.Username, issued.Password); err != ErrUnauthorized {
		t.Fatalf("revoked credential accepted: %v", err)
	}
	if len(store.List()) != 0 {
		t.Fatalf("revoked session still listed")
	}
}

func TestParseBasicAuthAndConnectTarget(t *testing.T) {
	token := base64.StdEncoding.EncodeToString([]byte("jpabc:secret"))
	user, pass, ok := ParseBasicAuth("Basic " + token)
	if !ok || user != "jpabc" || pass != "secret" {
		t.Fatalf("basic: %q %q %v", user, pass, ok)
	}
	if _, _, ok := ParseBasicAuth("Bearer nope"); ok {
		t.Fatal("bearer should not parse")
	}

	host, port, err := ParseConnectTarget("example.com:443")
	if err != nil || host != "example.com" || port != 443 {
		t.Fatalf("host: %s %d %v", host, port, err)
	}
	host, port, err = ParseConnectTarget("[2001:db8::1]:8443")
	if err != nil || host != "2001:db8::1" || port != 8443 {
		t.Fatalf("ipv6: %s %d %v", host, port, err)
	}
	if _, _, err := ParseConnectTarget("example.com"); err == nil {
		t.Fatal("missing port should fail")
	}
}

func TestIssueDoesNotOverwriteOnUsernameCollision(t *testing.T) {
	store := NewStore()
	orig := readRandom
	t.Cleanup(func() { readRandom = orig })
	readRandom = func(b []byte) (int, error) {
		for i := range b {
			b[i] = 0xAB
		}
		if len(b) == 16 {
			b[0] = byte(len(store.byID) + 1)
		}
		return len(b), nil
	}

	issued, err := store.Issue("agent-1", "Tokyo", 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Issue("agent-2", "Osaka", 8); err == nil {
		t.Fatal("expected username collision to fail")
	}
	got, err := store.Authenticate(issued.Username, issued.Password)
	if err != nil || got.AgentID != "agent-1" || got.ID != issued.ID {
		t.Fatalf("first session clobbered: %v %#v", err, got)
	}
	if len(store.byID) != 1 || len(store.byUser) != 1 {
		t.Fatalf("maps = %d ids %d users", len(store.byID), len(store.byUser))
	}
}

func TestIssueFailsWhenRandomFails(t *testing.T) {
	store := NewStore()
	orig := readRandom
	t.Cleanup(func() { readRandom = orig })
	readRandom = func([]byte) (int, error) {
		return 0, errors.New("rng unavailable")
	}
	if _, err := store.Issue("agent-1", "Tokyo", 7); err == nil {
		t.Fatal("expected rng error")
	}
	if len(store.byID) != 0 || len(store.byUser) != 0 {
		t.Fatal("failed issue stored a session")
	}
}
