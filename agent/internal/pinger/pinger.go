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

// Pinger runs ICMP measurements against targets
type Pinger struct {
	mu           sync.RWMutex
	targets      []protocol.TargetConfig
	isPrivileged bool
}

func NewPinger() *Pinger {
	// On Windows, privileged is typically false or true depending on raw socket. On Linux, unprivileged is default or fallback.
	isPriv := runtime.GOOS == "windows"
	return &Pinger{
		isPrivileged: isPriv,
	}
}

// UpdateTargets updates the active target list
func (p *Pinger) UpdateTargets(targets []protocol.TargetConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.targets = targets
}

// GetTargets returns a copy of current targets
func (p *Pinger) GetTargets() []protocol.TargetConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	copied := make([]protocol.TargetConfig, len(p.targets))
	copy(copied, p.targets)
	return copied
}

// PingAll runs a test round against all assigned targets concurrently
func (p *Pinger) PingAll(ctx context.Context) []protocol.SinglePingResult {
	targets := p.GetTargets()
	if len(targets) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	resultsChan := make(chan protocol.SinglePingResult, len(targets))
	sem := make(chan struct{}, 10) // Max 10 concurrent target tests

	for _, target := range targets {
		wg.Add(1)
		go func(t protocol.TargetConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res := p.PingTarget(ctx, t)
			resultsChan <- res
		}(target)
	}

	wg.Wait()
	close(resultsChan)

	var results []protocol.SinglePingResult
	for res := range resultsChan {
		results = append(results, res)
	}
	return results
}

// PingTarget sends ICMP packets to a single target and calculates RFC 3550 jitter & statistics
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

	pinger, err := probing.NewPinger(t.Host)
	if err != nil {
		res.ErrorMsg = err.Error()
		res.LossPct = 100.0
		return res
	}

	pinger.Count = pktCount
	pinger.Interval = 100 * time.Millisecond
	pinger.Timeout = time.Duration(pktCount)*120*time.Millisecond + 2*time.Second

	// Privilege setting
	pinger.SetPrivileged(p.isPrivileged)

	var rtts []float64
	var rttMu sync.Mutex
	pinger.OnRecv = func(pkt *probing.Packet) {
		rttMs := float64(pkt.Rtt.Microseconds()) / 1000.0
		rttMu.Lock()
		rtts = append(rtts, rttMs)
		rttMu.Unlock()
	}

	err = pinger.Run()
	if err != nil && runtime.GOOS == "linux" && !p.isPrivileged {
		// Fallback: try privileged mode if unprivileged failed
		pinger.SetPrivileged(true)
		err = pinger.Run()
	}

	stats := pinger.Statistics()
	res.PacketsSent = stats.PacketsSent
	res.PacketsRecv = stats.PacketsRecv
	res.LossPct = stats.PacketLoss

	if len(rtts) > 0 {
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

// calculateRFC3550Jitter implements RFC 3550 interarrival jitter algorithm
func calculateRFC3550Jitter(rtts []float64) float64 {
	if len(rtts) < 2 {
		return 0
	}

	var jitter float64 = 0
	for i := 1; i < len(rtts); i++ {
		d := math.Abs(rtts[i] - rtts[i-1])
		jitter += (d - jitter) / 16.0
	}
	return round(jitter)
}

func round(val float64) float64 {
	return math.Round(val*100) / 100
}
