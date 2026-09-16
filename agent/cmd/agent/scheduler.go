package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/guimc233/JustPing/agent/internal/client"
	"github.com/guimc233/JustPing/agent/internal/pinger"
	"github.com/guimc233/JustPing/agent/internal/traceroute"
	"github.com/guimc233/JustPing/shared/protocol"
)

const (
	TracerouteInterval = 12 * time.Hour
	MinTraceCooldown   = 30 * time.Minute
)

type TargetScheduler struct {
	ctx           context.Context
	cancel        context.CancelFunc
	pinger        *pinger.Pinger
	tracer        *traceroute.Tracer
	client        *client.Client
	mu            sync.Mutex
	running       map[string]context.CancelFunc
	targets       []protocol.TargetConfig
	traceTrigger  chan struct{}
	shiftTriggers chan string
	lastTracedAt  map[string]time.Time
	detectors     map[string]*traceroute.LatencyShiftDetector
}

func NewTargetScheduler(parentCtx context.Context, p *pinger.Pinger, c *client.Client) *TargetScheduler {
	ctx, cancel := context.WithCancel(parentCtx)
	s := &TargetScheduler{
		ctx:           ctx,
		cancel:        cancel,
		pinger:        p,
		tracer:        traceroute.NewTracer(),
		client:        c,
		running:       make(map[string]context.CancelFunc),
		targets:       make([]protocol.TargetConfig, 0),
		traceTrigger:  make(chan struct{}, 1),
		shiftTriggers: make(chan string, 64),
		lastTracedAt:  make(map[string]time.Time),
		detectors:     make(map[string]*traceroute.LatencyShiftDetector),
	}

	// Start the serial traceroute worker (12h cycle + persistent shift-triggered + 30m cooldown)
	go s.runSerialTracerouteLoop()

	return s
}

func (s *TargetScheduler) SyncTargets(targets []protocol.TargetConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, stop := range s.running {
		stop()
	}
	s.running = make(map[string]context.CancelFunc)
	s.targets = make([]protocol.TargetConfig, len(targets))
	copy(s.targets, targets)

	// Ensure detectors exist for all active targets
	for _, t := range targets {
		if _, exists := s.detectors[t.ID]; !exists {
			s.detectors[t.ID] = traceroute.NewLatencyShiftDetector(3)
		}
	}

	for _, t := range targets {
		targetCtx, targetCancel := context.WithCancel(s.ctx)
		s.running[t.ID] = targetCancel
		// Launch concurrent worker for each target independently for ping
		go s.runTargetLoop(targetCtx, t)
	}

	// Trigger initial traceroute for new targets if not already scheduled
	select {
	case s.traceTrigger <- struct{}{}:
	default:
	}
}

func (s *TargetScheduler) runTargetLoop(ctx context.Context, t protocol.TargetConfig) {
	// Schedule: exactly 1 ICMP packet every IntervalSec (defaults to 30 seconds)
	intervalSec := t.IntervalSec
	if intervalSec <= 0 {
		intervalSec = 30
	}
	interval := time.Duration(intervalSec) * time.Second

	// Send initial single probe immediately upon target assignment
	res := s.pinger.PingTargetOnce(ctx, t)
	s.client.QueueReport(protocol.PingReportPayload{Results: []protocol.SinglePingResult{res}})
	s.checkLatencyShift(t, res)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send 1 single ICMP packet every 30s, aggregate loss/latency sliding window
			res := s.pinger.PingTargetOnce(ctx, t)
			s.client.QueueReport(protocol.PingReportPayload{Results: []protocol.SinglePingResult{res}})
			s.checkLatencyShift(t, res)
		}
	}
}

func (s *TargetScheduler) checkLatencyShift(t protocol.TargetConfig, res protocol.SinglePingResult) {
	if t.DisableRoute || res.AvgRTT <= 0 {
		return
	}

	s.mu.Lock()
	det := s.detectors[t.ID]
	s.mu.Unlock()

	if det == nil {
		return
	}

	if hasShift := det.Feed(res.AvgRTT); hasShift {
		log.Printf("[Traceroute] Target %s (%s) detected persistent latency shift (sustained at %.1fms)! Queueing route trace...\n",
			t.Name, t.Host, res.AvgRTT)
		select {
		case s.shiftTriggers <- t.ID:
		default:
		}
	}
}

// runSerialTracerouteLoop runs traceroutes strictly sequentially across targets:
// 1. Every 12 hours periodically
// 2. Triggered immediately upon confirmed persistent latency shifts
// 3. Enforcing a strict minimum 30-minute cooldown per target
func (s *TargetScheduler) runSerialTracerouteLoop() {
	ticker := time.NewTicker(TracerouteInterval)
	defer ticker.Stop()

	// Initial grace period before the first traceroute
	select {
	case <-s.ctx.Done():
		return
	case <-time.After(5 * time.Second):
		s.executeSerialTraces(false)
	case <-s.traceTrigger:
		s.executeSerialTraces(false)
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			log.Printf("[Traceroute] 12-hour periodic traceroute schedule triggered.\n")
			s.executeSerialTraces(false)
		case <-s.traceTrigger:
			s.executeSerialTraces(false)
		case targetID := <-s.shiftTriggers:
			s.executeSingleShiftTrace(targetID)
		}
	}
}

func (s *TargetScheduler) executeSingleShiftTrace(targetID string) {
	s.mu.Lock()
	var target *protocol.TargetConfig
	for _, t := range s.targets {
		if t.ID == targetID {
			copied := t
			target = &copied
			break
		}
	}
	last := s.lastTracedAt[targetID]
	s.mu.Unlock()

	if target == nil || target.DisableRoute {
		return
	}

	// 30-minute minimum cooldown per target
	if time.Since(last) < MinTraceCooldown {
		log.Printf("[Traceroute] Target %s shifted but is within 30-minute cooldown (last traced %s ago). Skipping.\n",
			target.Name, time.Since(last).Truncate(time.Second))
		return
	}

	log.Printf("[Traceroute] [Shift-Triggered] Tracing route to %s (%s) (max 30 hops)...\n", target.Name, target.Host)
	report := s.tracer.TraceTarget(s.ctx, *target)
	s.client.SendTracerouteReport(report)

	s.mu.Lock()
	s.lastTracedAt[targetID] = time.Now()
	s.mu.Unlock()
}

func (s *TargetScheduler) executeSerialTraces(force bool) {
	s.mu.Lock()
	targetsCopy := make([]protocol.TargetConfig, len(s.targets))
	copy(targetsCopy, s.targets)
	s.mu.Unlock()

	if len(targetsCopy) == 0 {
		return
	}

	log.Printf("[Traceroute] Checking serial route trace across %d targets (cooldown: 30m, interval: 12h)...\n", len(targetsCopy))

	// Strictly serial execution: one target after another, NO concurrency
	for i, t := range targetsCopy {
		if s.ctx.Err() != nil {
			return
		}

		if t.DisableRoute {
			continue
		}

		s.mu.Lock()
		last := s.lastTracedAt[t.ID]
		s.mu.Unlock()

		// Skip if traced within minimum 30 minutes cooldown
		if !force && time.Since(last) < MinTraceCooldown {
			continue
		}

		log.Printf("[Traceroute] [%d/%d] Tracing route to %s (%s) (max 30 hops)...\n", i+1, len(targetsCopy), t.Name, t.Host)
		report := s.tracer.TraceTarget(s.ctx, t)
		s.client.SendTracerouteReport(report)

		s.mu.Lock()
		s.lastTracedAt[t.ID] = time.Now()
		s.mu.Unlock()

		// Brief delay between targets to prevent packet bursts
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *TargetScheduler) Stop() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stop := range s.running {
		stop()
	}
	s.running = make(map[string]context.CancelFunc)
}
