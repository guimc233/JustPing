package pinger

import (
	"context"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/guimc233/JustPing/shared/protocol"
	probing "github.com/prometheus-community/pro-bing"
)

type Pinger struct {
	mu           sync.RWMutex
	targets      []protocol.TargetConfig
	isPrivileged bool
	privMu       sync.RWMutex
	windows      sync.Map // map[string]*TargetWindow
}

func NewPinger() *Pinger {
	return &Pinger{
		isPrivileged: runtime.GOOS == "windows",
	}
}

func (p *Pinger) GetOrCreateWindow(targetID string, windowSize int) *TargetWindow {
	if windowSize <= 0 {
		windowSize = 20
	}
	actual, _ := p.windows.LoadOrStore(targetID, NewTargetWindow(windowSize))
	return actual.(*TargetWindow)
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

// PingTargetOnce sends 1 single ICMP packet, registers it in the target's sliding window,
// and returns the aggregated window summary metric.
func (p *Pinger) PingTargetOnce(ctx context.Context, t protocol.TargetConfig) protocol.SinglePingResult {
	// Window size: default to t.PacketCount (e.g. 15-20 samples = 7.5-10 minutes of history at 30s intervals)
	windowSize := t.PacketCount
	if windowSize <= 0 {
		windowSize = 20
	}

	win := p.GetOrCreateWindow(t.ID, windowSize)

	// Send single ICMP packet with a 3s timeout
	stat := p.ExecuteSinglePing(ctx, t.Host, 3*time.Second)
	win.Add(stat)

	summary := win.Summary()

	res := protocol.SinglePingResult{
		TargetID:    t.ID,
		TargetHost:  t.Host,
		Timestamp:   stat.Timestamp,
		PacketsSent: summary.TotalSent,
		PacketsRecv: summary.TotalRecv,
		LossPct:     summary.LossPct,
		MinRTT:      summary.MinRTT,
		MaxRTT:      summary.MaxRTT,
		AvgRTT:      summary.AvgRTT,
		Jitter:      summary.Jitter,
		StdDev:      summary.StdDev,
		ErrorMsg:    summary.LatestError,
	}

	return res
}

// PingTarget sends packetCount packets (legacy/multi-ping compatibility mode)
func (p *Pinger) PingTarget(ctx context.Context, t protocol.TargetConfig) protocol.SinglePingResult {
	return p.PingTargetOnce(ctx, t)
}

func (p *Pinger) executePing(ctx context.Context, host string, count int, privileged bool) (*probing.Statistics, []float64, error) {
	pinger, err := probing.NewPinger(host)
	if err != nil {
		return nil, nil, err
	}

	pinger.Count = count
	pinger.Interval = 1 * time.Second
	pinger.Timeout = time.Duration(count)*1100*time.Millisecond + 3*time.Second
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
