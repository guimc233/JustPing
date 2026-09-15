package pinger

import (
	"context"
	"math"
	"runtime"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/guimc233/JustPing/shared/protocol"
)

type Pinger struct {
	mu           sync.RWMutex
	targets      []protocol.TargetConfig
	isPrivileged bool
	privMu       sync.RWMutex
}

func NewPinger() *Pinger {
	return &Pinger{
		isPrivileged: runtime.GOOS == "windows",
	}
}

func (p *Pinger) SetPrivileged(priv bool) {
	p.privMu.Lock()
	defer p.privMu.Unlock()
	p.isPrivileged = priv
}

func (p *Pinger) IsPrivileged() bool {
	p.privMu.RLock()
	defer p.privMu.RUnlock()
	return p.isPrivileged
}

func (p *Pinger) UpdateTargets(targets []protocol.TargetConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.targets = targets
}

func (p *Pinger) GetTargets() []protocol.TargetConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	copied := make([]protocol.TargetConfig, len(p.targets))
	copy(copied, p.targets)
	return copied
}

func (p *Pinger) PingTarget(ctx context.Context, t protocol.TargetConfig) protocol.SinglePingResult {
	pktCount := t.PacketCount
	if pktCount <= 0 {
		pktCount = 15
	}
	if pktCount > 50 {
		pktCount = 50
	}

	res := protocol.SinglePingResult{
		TargetID:    t.ID,
		TargetHost:  t.Host,
		Timestamp:   time.Now().UTC(),
		PacketsSent: pktCount,
	}

	priv := p.IsPrivileged()
	stats, rtts, err := p.executePing(ctx, t.Host, pktCount, priv)
	if err != nil && runtime.GOOS == "linux" && !priv {
		// Fresh retry with privileged mode
		var err2 error
		stats, rtts, err2 = p.executePing(ctx, t.Host, pktCount, true)
		if err2 == nil || (stats != nil && stats.PacketsRecv > 0) {
			p.SetPrivileged(true) // Sticky privilege flag
			err = err2
		}
	}

	if stats != nil {
		res.PacketsSent = stats.PacketsSent
		res.PacketsRecv = stats.PacketsRecv
		res.LossPct = stats.PacketLoss
	}

	if len(rtts) > 0 && stats != nil {
		res.MinRTT = round(float64(stats.MinRtt.Microseconds()) / 1000.0)
		res.MaxRTT = round(float64(stats.MaxRtt.Microseconds()) / 1000.0)
		res.AvgRTT = round(float64(stats.AvgRtt.Microseconds()) / 1000.0)
		res.StdDev = round(float64(stats.StdDevRtt.Microseconds()) / 1000.0)
		res.Jitter = calculateRFC3550Jitter(rtts)
	} else {
		res.LossPct = 100.0
		if err != nil {
			res.ErrorMsg = err.Error()
		} else {
			res.ErrorMsg = "All packets timed out"
		}
	}

	return res
}

func (p *Pinger) executePing(ctx context.Context, host string, count int, privileged bool) (*probing.Statistics, []float64, error) {
	pinger, err := probing.NewPinger(host)
	if err != nil {
		return nil, nil, err
	}

	pinger.Count = count
	pinger.Interval = 100 * time.Millisecond
	pinger.Timeout = time.Duration(count)*120*time.Millisecond + 2*time.Second
	pinger.SetPrivileged(privileged)

	var rtts []float64
	var rttMu sync.Mutex
	pinger.OnRecv = func(pkt *probing.Packet) {
		rttMs := float64(pkt.Rtt.Microseconds()) / 1000.0
		rttMu.Lock()
		rtts = append(rtts, rttMs)
		rttMu.Unlock()
	}

	err = pinger.RunWithContext(ctx)
	return pinger.Statistics(), rtts, err
}

func calculateRFC3550Jitter(rtts []float64) float64 {
	if len(rtts) < 2 {
		return 0
	}
	var jitter float64
	for i := 1; i < len(rtts); i++ {
		d := math.Abs(rtts[i] - rtts[i-1])
		jitter += (d - jitter) / 16.0
	}
	return round(jitter)
}

func round(val float64) float64 {
	return math.Round(val*100) / 100
}
