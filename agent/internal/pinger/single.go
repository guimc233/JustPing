package pinger

import (
	"context"
	"errors"
	"math"
	"net"
	"runtime"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// SinglePingStat records the result of a single 30s ICMP packet transmission
type SinglePingStat struct {
	Timestamp time.Time
	Success   bool
	RTT       float64 // ms
	ErrorMsg  string
}

// TargetWindow tracks the sliding window of ping samples for a given target
type TargetWindow struct {
	mu      sync.Mutex
	history []SinglePingStat
	maxKeep int // maximum samples kept (e.g. 20 samples = 10 minutes at 30s interval)
}

func NewTargetWindow(maxKeep int) *TargetWindow {
	if maxKeep <= 0 {
		maxKeep = 20
	}
	return &TargetWindow{
		history: make([]SinglePingStat, 0, maxKeep),
		maxKeep: maxKeep,
	}
}

func (w *TargetWindow) Add(stat SinglePingStat) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.history = append(w.history, stat)
	if len(w.history) > w.maxKeep {
		w.history = w.history[len(w.history)-w.maxKeep:]
	}
}

// WindowSummary aggregates the sliding window stats
type WindowSummary struct {
	TotalSent   int
	TotalRecv   int
	LossPct     float64
	MinRTT      float64
	MaxRTT      float64
	AvgRTT      float64
	Jitter      float64
	StdDev      float64
	LatestRTT   float64
	LatestError string
}

func (w *TargetWindow) Summary() WindowSummary {
	w.mu.Lock()
	defer w.mu.Unlock()

	total := len(w.history)
	if total == 0 {
		return WindowSummary{}
	}

	var recv int
	var rtts []float64
	var sumRTT float64
	minRTT := math.MaxFloat64
	maxRTT := 0.0
	var lastErr string

	for _, s := range w.history {
		if s.Success {
			recv++
			rtts = append(rtts, s.RTT)
			sumRTT += s.RTT
			if s.RTT < minRTT {
				minRTT = s.RTT
			}
			if s.RTT > maxRTT {
				maxRTT = s.RTT
			}
		} else if s.ErrorMsg != "" {
			lastErr = s.ErrorMsg
		}
	}

	lossPct := float64(total-recv) / float64(total) * 100.0

	summary := WindowSummary{
		TotalSent:   total,
		TotalRecv:   recv,
		LossPct:     round(lossPct),
		LatestError: lastErr,
	}

	if recv > 0 {
		summary.MinRTT = round(minRTT)
		summary.MaxRTT = round(maxRTT)
		avg := sumRTT / float64(recv)
		summary.AvgRTT = round(avg)
		summary.LatestRTT = round(rtts[len(rtts)-1])

		// Calculate StdDev
		var varianceSum float64
		for _, r := range rtts {
			d := r - avg
			varianceSum += d * d
		}
		summary.StdDev = round(math.Sqrt(varianceSum / float64(recv)))
		summary.Jitter = calculateRFC3550Jitter(rtts)
	} else {
		summary.LossPct = 100.0
		if summary.LatestError == "" {
			summary.LatestError = "All packets timed out"
		}
	}

	return summary
}

// ExecuteSinglePing sends exactly 1 ICMP echo packet to the host
func (p *Pinger) ExecuteSinglePing(ctx context.Context, host string, timeout time.Duration) SinglePingStat {
	now := time.Now().UTC()
	priv := p.IsPrivileged()

	stat, err := p.pingOnce(ctx, host, timeout, priv)
	if err != nil && runtime.GOOS == "linux" && !priv {
		// Retry with privileged mode if initial ping failed on Linux
		var err2 error
		stat, err2 = p.pingOnce(ctx, host, timeout, true)
		if err2 == nil || stat.Success {
			p.SetPrivileged(true)
			err = err2
		}
	}

	if err != nil {
		stat.ErrorMsg = err.Error()
	}
	stat.Timestamp = now
	return stat
}

func (p *Pinger) pingOnce(ctx context.Context, host string, timeout time.Duration, privileged bool) (SinglePingStat, error) {
	stat := SinglePingStat{}

	// Validate or resolve host first with context
	lookupCtx, lookupCancel := context.WithTimeout(ctx, 3*time.Second)
	defer lookupCancel()

	var resolver net.Resolver
	addrs, err := resolver.LookupHost(lookupCtx, host)
	if err != nil || len(addrs) == 0 {
		stat.Success = false
		if err != nil {
			stat.ErrorMsg = err.Error()
			return stat, err
		}
		stat.ErrorMsg = "host resolution failed"
		return stat, errors.New(stat.ErrorMsg)
	}

	pinger, err := probing.NewPinger(addrs[0])
	if err != nil {
		stat.Success = false
		stat.ErrorMsg = err.Error()
		return stat, err
	}

	pinger.Count = 1
	pinger.Interval = time.Second
	pinger.Timeout = timeout
	pinger.SetPrivileged(privileged)

	var recvRTT float64
	var recvOk bool
	pinger.OnRecv = func(pkt *probing.Packet) {
		recvRTT = float64(pkt.Rtt.Microseconds()) / 1000.0
		recvOk = true
	}

	err = pinger.RunWithContext(ctx)
	if err != nil {
		stat.Success = false
		stat.ErrorMsg = err.Error()
		return stat, err
	}

	stats := pinger.Statistics()
	if recvOk || (stats != nil && stats.PacketsRecv > 0) {
		stat.Success = true
		if recvOk {
			stat.RTT = recvRTT
		} else if stats != nil && stats.AvgRtt > 0 {
			stat.RTT = float64(stats.AvgRtt.Microseconds()) / 1000.0
		}
	} else {
		stat.Success = false
		stat.ErrorMsg = "Packet timed out"
	}

	return stat, nil
}
