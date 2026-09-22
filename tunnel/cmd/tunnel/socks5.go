package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"time"
)

// Minimal SOCKS5 (RFC 1928) server: no authentication, CONNECT command only.
// That is enough to carry arbitrary TCP, which is what makes the client usable
// by tools that do not speak an HTTP proxy protocol.
const (
	socksVersion    = 0x05
	socksNoAuth     = 0x00
	socksNoAccept   = 0xFF
	socksCmdConnect = 0x01

	socksAtypIPv4   = 0x01
	socksAtypDomain = 0x03
	socksAtypIPv6   = 0x04

	socksReplyOK               = 0x00
	socksReplyHostUnreachable  = 0x04
	socksReplyCmdNotSupported  = 0x07
	socksReplyAtypNotSupported = 0x08
)

// socksNegotiationTimeout bounds the greeting and request exchange.
const socksNegotiationTimeout = 30 * time.Second

// handleSOCKS5 serves one SOCKS5 connection end to end. br holds bytes already
// read while sniffing the protocol, so the greeting is read from there.
func (p *LocalProxy) handleSOCKS5(conn net.Conn, br *bufio.Reader) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(socksNegotiationTimeout))

	if err := socksNegotiate(conn, br); err != nil {
		return
	}
	target, err := socksReadRequest(conn, br)
	if err != nil {
		// socksReadRequest already replied where a reply is possible.
		return
	}

	// The handshake is over; the tunnel itself may outlive any fixed deadline.
	_ = conn.SetDeadline(time.Time{})

	upstream, _, err := p.dialer.Dial(target)
	if err != nil {
		_ = socksWriteReply(conn, socksReplyHostUnreachable)
		p.logf("socks %s failed: %v", target, err)
		return
	}

	if err := socksWriteReply(conn, socksReplyOK); err != nil {
		_ = upstream.Close()
		return
	}

	// Bytes the client pipelined after its request must reach the target first.
	if br.Buffered() > 0 {
		if _, err := io.CopyN(upstream, br, int64(br.Buffered())); err != nil {
			_ = upstream.Close()
			return
		}
	}

	p.logf("socks %s open", target)
	relay(conn, upstream)
}

// socksNegotiate performs the method-selection handshake. Only "no auth" is
// offered, which is safe because the listener is loopback-only.
func socksNegotiate(conn net.Conn, br *bufio.Reader) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(br, header); err != nil {
		return err
	}
	if header[0] != socksVersion {
		return errors.New("not socks5")
	}

	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(br, methods); err != nil {
		return err
	}
	offered := false
	for _, m := range methods {
		if m == socksNoAuth {
			offered = true
			break
		}
	}
	if !offered {
		_, _ = conn.Write([]byte{socksVersion, socksNoAccept})
		return errors.New("no acceptable auth method")
	}

	_, err := conn.Write([]byte{socksVersion, socksNoAuth})
	return err
}

// socksReadRequest parses the CONNECT request and returns "host:port".
func socksReadRequest(conn net.Conn, br *bufio.Reader) (string, error) {
	head := make([]byte, 3)
	if _, err := io.ReadFull(br, head); err != nil {
		return "", err
	}
	if head[0] != socksVersion {
		return "", errors.New("bad version")
	}
	if head[1] != socksCmdConnect {
		// BIND and UDP ASSOCIATE are not supported.
		_ = socksWriteReply(conn, socksReplyCmdNotSupported)
		return "", errors.New("unsupported command")
	}

	target, err := socksReadAddr(br, head[2])
	if err != nil {
		_ = socksWriteReply(conn, socksReplyAtypNotSupported)
		return "", err
	}
	return target, nil
}

// socksReadAddr reads an address in the SOCKS ATYP encoding.
func socksReadAddr(r io.Reader, atyp byte) (string, error) {
	var host string
	switch atyp {
	case socksAtypIPv4:
		buf := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		host = net.IP(buf).String()
	case socksAtypIPv6:
		buf := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		host = net.IP(buf).String()
	case socksAtypDomain:
		length := make([]byte, 1)
		if _, err := io.ReadFull(r, length); err != nil {
			return "", err
		}
		buf := make([]byte, int(length[0]))
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		host = string(buf)
	default:
		return "", errors.New("unsupported address type")
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(r, portBuf); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portBuf)
	if host == "" || port == 0 {
		return "", errors.New("incomplete address")
	}
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

// socksWriteReply sends a reply with an unspecified bound address, which is
// allowed and is what most clients expect when the proxy has no meaningful
// local address to report.
func socksWriteReply(w io.Writer, reply byte) error {
	_, err := w.Write([]byte{
		socksVersion, reply, 0x00,
		socksAtypIPv4, 0, 0, 0, 0,
		0, 0,
	})
	return err
}
