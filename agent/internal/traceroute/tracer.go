package traceroute

import (
	"context"
	"math"
	"math/rand"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/guimc233/JustPing/shared/protocol"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	MaxHops        = 30
	ProbesPerHop   = 3
	ProbeTimeout   = 1200 * time.Millisecond
	ResolveTimeout = 3 * time.Second
)

type Tracer struct {
	mu sync.Mutex
}

func NewTracer() *Tracer {
	return &Tracer{}
}

// TraceTarget executes an ICMP traceroute up to max 30 hops sequentially.
func (t *Tracer) TraceTarget(ctx context.Context, target protocol.TargetConfig) protocol.TracerouteReportPayload {
	t.mu.Lock()
	defer t.mu.Unlock()

	startTime := time.Now().UTC()
	report := protocol.TracerouteReportPayload{
		TargetID:   target.ID,
		TargetHost: target.Host,
		Timestamp:  startTime,
		Hops:       make([]protocol.TracerouteHop, 0, MaxHops),
	}

	// Resolve destination IP
	resolveCtx, resolveCancel := context.WithTimeout(ctx, ResolveTimeout)
	defer resolveCancel()

	ips, err := net.DefaultResolver.LookupIP(resolveCtx, "ip4", target.Host)
	if err != nil || len(ips) == 0 {
		report.DurationMs = time.Since(startTime).Milliseconds()
		return report
	}

	destIP := ips[0].String()
	report.ResolvedIP = destIP

	// Listen for ICMP packets
	c, isUDP, err := listenICMP()
	if err != nil {
		report.DurationMs = time.Since(startTime).Milliseconds()
		return report
	}
	defer c.Close()

	pConn := c.IPv4PacketConn()
	pid := os.Getpid() & 0xffff

	reached := false

	// Trace TTL from 1 up to MaxHops (30)
	for ttl := 1; ttl <= MaxHops; ttl++ {
		if ctx.Err() != nil {
			break
		}

		hop := protocol.TracerouteHop{
			TTL:  ttl,
			RTTs: make([]float64, 0, ProbesPerHop),
		}

		var hopIP string
		var recvCount int

		for attempt := 0; attempt < ProbesPerHop; attempt++ {
			if ctx.Err() != nil {
				break
			}

			// Generate unique sequence number
			seq := (ttl << 8) | (attempt & 0xff) | (rand.Intn(0x7f) << 4)

			// Set TTL on socket
			if pConn != nil {
				_ = pConn.SetTTL(ttl)
			}

			var dstAddr net.Addr
			if isUDP {
				dstAddr = &net.UDPAddr{IP: ips[0], Port: 33434 + ttl}
			} else {
				dstAddr = &net.IPAddr{IP: ips[0]}
			}

			echoMsg := icmp.Message{
				Type: ipv4.ICMPTypeEcho,
				Code: 0,
				Body: &icmp.Echo{
					ID:   pid,
					Seq:  seq,
					Data: []byte("JustPingTraceroute"),
				},
			}

			wb, err := echoMsg.Marshal(nil)
			if err != nil {
				continue
			}

			sendTime := time.Now()
			if _, err := c.WriteTo(wb, dstAddr); err != nil {
				continue
			}

			// Wait for reply
			replyIP, rtt, isDestination, err := waitReply(c, pid, seq, destIP, sendTime, ProbeTimeout)
			if err == nil && replyIP != "" {
				hopIP = replyIP
				hop.RTTs = append(hop.RTTs, rtt)
				recvCount++
				if isDestination || replyIP == destIP {
					reached = true
				}
			}
		}

		hop.IP = hopIP
		hop.LossPct = roundFloat(float64(ProbesPerHop-recvCount) / float64(ProbesPerHop) * 100.0)

		if len(hop.RTTs) > 0 {
			var sum float64
			for _, r := range hop.RTTs {
				sum += r
			}
			hop.AvgRTT = roundFloat(sum / float64(len(hop.RTTs)))
		}

		// Reverse DNS lookup for responding IP
		if hop.IP != "" {
			hop.Hostname = lookupHostname(ctx, hop.IP)
		}

		report.Hops = append(report.Hops, hop)

		if reached {
			report.Reached = true
			break
		}
	}

	report.DurationMs = time.Since(startTime).Milliseconds()
	return report
}

func listenICMP() (*icmp.PacketConn, bool, error) {
	// Try raw ip4:icmp first
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err == nil {
		return c, false, nil
	}

	// If unprivileged on Linux/Darwin, try udp4
	if runtime.GOOS != "windows" {
		if cUDP, errUDP := icmp.ListenPacket("udp4", "0.0.0.0"); errUDP == nil {
			return cUDP, true, nil
		}
	}

	return nil, false, err
}

func waitReply(c *icmp.PacketConn, expectedID, expectedSeq int, destIP string, sendTime time.Time, timeout time.Duration) (string, float64, bool, error) {
	deadline := sendTime.Add(timeout)
	_ = c.SetReadDeadline(deadline)

	rb := make([]byte, 1500)
	for {
		if time.Now().After(deadline) {
			return "", 0, false, context.DeadlineExceeded
		}

		n, peer, err := c.ReadFrom(rb)
		if err != nil {
			return "", 0, false, err
		}

		rtt := roundFloat(float64(time.Since(sendTime).Microseconds()) / 1000.0)
		peerIP := extractIP(peer)

		msg, err := icmp.ParseMessage(ipv4.ICMPTypeEcho.Protocol(), rb[:n])
		if err != nil {
			continue
		}

		switch msg.Type {
		case ipv4.ICMPTypeTimeExceeded:
			// Intermediate hop TTL exceeded
			return peerIP, rtt, false, nil

		case ipv4.ICMPTypeEchoReply:
			// Final destination reached
			echo, ok := msg.Body.(*icmp.Echo)
			if ok && (expectedID == 0 || echo.ID == expectedID) {
				return peerIP, rtt, true, nil
			}
			if peerIP == destIP {
				return peerIP, rtt, true, nil
			}

		case ipv4.ICMPTypeDestinationUnreachable:
			// Destination unreachable (e.g. port unreachable for UDP probe)
			return peerIP, rtt, peerIP == destIP, nil
		}
	}
}

func extractIP(peer net.Addr) string {
	if peer == nil {
		return ""
	}
	addr := peer.String()
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

func lookupHostname(ctx context.Context, ipStr string) string {
	lookupCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(lookupCtx, ipStr)
	if err != nil || len(names) == 0 {
		return ""
	}
	return strings.TrimSuffix(names[0], ".")
}

func roundFloat(val float64) float64 {
	return math.Round(val*100) / 100
}
