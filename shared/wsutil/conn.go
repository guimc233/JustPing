// Package wsutil adapts a WebSocket connection to net.Conn so a WebSocket can
// carry a raw byte stream.
//
// It exists because a reverse proxy in front of the Host commonly refuses the
// HTTP CONNECT method (nginx answers 405 Not Allowed), while it does forward
// WebSocket upgrades. Tunnelling over WebSocket therefore works on the same
// port and through the same TLS listener, on both ends of the relay.
package wsutil

import (
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

// Conn presents a binary-message WebSocket as a net.Conn byte stream.
//
// Each Write becomes one binary message and Read returns the payloads of
// received binary messages in order. It is not safe for concurrent writes, which
// matches net.Conn usage in a byte-pump: one goroutine reads, one writes.
//
// Deadline methods are no-ops. WebSocket has no notion of a byte-level deadline,
// and gorilla already applies its own write deadline, so callers that need a hard
// lifetime bound must enforce it themselves (the tunnel relays cancel on the
// credential's expiry instead).
type Conn struct {
	ws     *websocket.Conn
	reader io.Reader
}

// NewConn wraps ws. The caller must not use ws directly afterwards.
func NewConn(ws *websocket.Conn) *Conn { return &Conn{ws: ws} }

// Read returns bytes from the current binary message, moving to the next message
// when one is exhausted. Control frames are handled by the WebSocket library.
func (c *Conn) Read(p []byte) (int, error) {
	for {
		if c.reader == nil {
			_, r, err := c.ws.NextReader()
			if err != nil {
				return 0, err
			}
			c.reader = r
		}

		n, err := c.reader.Read(p)
		if err == io.EOF {
			c.reader = nil
			if n > 0 {
				return n, nil
			}
			// Message boundary: try the next message. An empty message is not
			// EOF for the connection.
			continue
		}
		return n, err
	}
}

// Write sends p as one binary message.
func (c *Conn) Write(p []byte) (int, error) {
	w, err := c.ws.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return 0, err
	}
	n, err := w.Write(p)
	if closeErr := w.Close(); err == nil {
		err = closeErr
	}
	return n, err
}

// Close closes the underlying WebSocket.
func (c *Conn) Close() error { return c.ws.Close() }

// LocalAddr reports the local address of the underlying connection.
func (c *Conn) LocalAddr() net.Addr { return c.ws.LocalAddr() }

// RemoteAddr reports the peer address of the underlying connection.
func (c *Conn) RemoteAddr() net.Addr { return c.ws.RemoteAddr() }

// SetDeadline is a no-op; see the type comment.
func (c *Conn) SetDeadline(time.Time) error { return nil }

// SetReadDeadline is a no-op; see the type comment.
func (c *Conn) SetReadDeadline(time.Time) error { return nil }

// SetWriteDeadline is a no-op; see the type comment.
func (c *Conn) SetWriteDeadline(time.Time) error { return nil }
