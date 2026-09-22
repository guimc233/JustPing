package proxy

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SessionTTL is the lifetime of an admin-issued HTTPS proxy credential.
const SessionTTL = 10 * time.Minute

var ErrUnauthorized = errors.New("unauthorized")

// Session is an admin-issued proxy login. The password hash never leaves this package.
type Session struct {
	ID        string
	AgentID   string
	AgentName string
	Username  string
	CreatedBy uint
	CreatedAt time.Time
	ExpiresAt time.Time

	passwordHash string
	revoked      bool
}

// IssuedSession is returned once, with the plaintext password.
type IssuedSession struct {
	Session
	Password string
}

// Store keeps short-lived proxy credentials in memory.
type Store struct {
	mu     sync.Mutex
	byID   map[string]*Session
	byUser map[string]*Session
	now    func() time.Time
}

func NewStore() *Store {
	return &Store{
		byID:   make(map[string]*Session),
		byUser: make(map[string]*Session),
		now:    time.Now,
	}
}

// Issue creates a credential bound to one probe. The password is returned only here.
func (s *Store) Issue(agentID, agentName string, userID uint) (IssuedSession, error) {
	passRaw := make([]byte, 24)
	if _, err := rand.Read(passRaw); err != nil {
		return IssuedSession{}, err
	}
	password := hex.EncodeToString(passRaw)
	sum := sha256.Sum256([]byte(password))

	id, err := randomHex(16)
	if err != nil {
		return IssuedSession{}, err
	}
	now := s.now()
	sess := &Session{
		ID:           id,
		AgentID:      agentID,
		AgentName:    agentName,
		CreatedBy:    userID,
		CreatedAt:    now,
		ExpiresAt:    now.Add(SessionTTL),
		passwordHash: hex.EncodeToString(sum[:]),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(now)
	if _, exists := s.byID[sess.ID]; exists {
		return IssuedSession{}, errors.New("failed to allocate proxy session")
	}
	allocated := false
	for range 5 {
		suffix, err := randomHex(6)
		if err != nil {
			return IssuedSession{}, err
		}
		username := "jp" + suffix
		if _, exists := s.byUser[username]; exists {
			continue
		}
		sess.Username = username
		allocated = true
		break
	}
	if !allocated {
		return IssuedSession{}, errors.New("failed to allocate proxy username")
	}
	s.byID[sess.ID] = sess
	s.byUser[sess.Username] = sess
	return IssuedSession{Session: *sess, Password: password}, nil
}

// Authenticate accepts a username and password that are still inside the 10 minute window.
func (s *Store) Authenticate(username, password string) (*Session, error) {
	sum := sha256.Sum256([]byte(password))
	got := hex.EncodeToString(sum[:])
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.byUser[username]
	if sess == nil || sess.revoked || !now.Before(sess.ExpiresAt) {
		return nil, ErrUnauthorized
	}
	if subtle.ConstantTimeCompare([]byte(got), []byte(sess.passwordHash)) != 1 {
		return nil, ErrUnauthorized
	}
	cp := *sess
	return &cp, nil
}

// List returns credentials that can still authenticate.
func (s *Store) List() []Session {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(now)
	out := make([]Session, 0, len(s.byID))
	for _, sess := range s.byID {
		out = append(out, *sess)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ExpiresAt.Before(out[j].ExpiresAt)
	})
	return out
}

// Revoke invalidates a credential immediately. Expired or unknown ids return false.
func (s *Store) Revoke(id string) bool {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.byID[id]
	if sess == nil || sess.revoked || !now.Before(sess.ExpiresAt) {
		return false
	}
	sess.revoked = true
	delete(s.byID, id)
	if cur := s.byUser[sess.Username]; cur == sess {
		delete(s.byUser, sess.Username)
	}
	return true
}

func (s *Store) purgeLocked(now time.Time) {
	for id, sess := range s.byID {
		if sess.revoked || !now.Before(sess.ExpiresAt) {
			delete(s.byID, id)
			if cur := s.byUser[sess.Username]; cur == sess {
				delete(s.byUser, sess.Username)
			}
		}
	}
}

// readRandom is crypto/rand.Read unless a test replaces it.
var readRandom = rand.Read

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := readRandom(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ParseBasicAuth parses a Proxy-Authorization Basic header.
func ParseBasicAuth(header string) (string, string, bool) {
	const prefix = "basic "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[len(prefix):]))
	if err != nil {
		return "", "", false
	}
	user, pass, ok := strings.Cut(string(raw), ":")
	if !ok || user == "" || pass == "" {
		return "", "", false
	}
	return user, pass, true
}

// ParseConnectTarget parses a CONNECT host:port, including bracketed IPv6.
func ParseConnectTarget(raw string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err != nil {
		return "", 0, errors.New("invalid connect target")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 || host == "" {
		return "", 0, errors.New("invalid connect target")
	}
	return host, port, nil
}
