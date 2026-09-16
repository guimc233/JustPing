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

const TracerouteInterval = 15 * time.Minute

type TargetScheduler struct {
	ctx          context.Context
	cancel       context.CancelFunc
	pinger       *pinger.Pinger
	tracer       *traceroute.Tracer
	client       *client.Client
	mu           sync.Mutex
	running      map[string]context.CancelFunc
	targets      []protocol.TargetConfig
	traceTrigger chan struct{}
}

func NewTargetScheduler(parentCtx context.Context, p *pinger.Pinger, c *client.Client) *TargetScheduler {
	ctx, cancel := context.WithCancel(parentCtx)
	s := &TargetScheduler{
		ctx:          ctx,
		cancel:       cancel,
		pinger:       p,
		tracer:       traceroute.NewTracer(),
		client:       c,
		running:      make(map[string]context.CancelFunc),
		targets:      make([]protocol.TargetConfig, 0),
		traceTrigger: make(chan struct{}, 1),
	}

	// Start the serial 15-minute traceroute worker
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

	for _, t := range targets {
		targetCtx, targetCancel := context.WithCancel(s.ctx)
		s.running[t.ID] = targetCancel
		// Launch concurrent worker for each target independently for ping
		go s.runTargetLoop(targetCtx, t)
	}

	// Trigger immediate traceroute for new targets if not already scheduled
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
		}
	}
}

// runSerialTracerouteLoop runs traceroutes strictly sequentially across all targets every 15 minutes.
func (s *TargetScheduler) runSerialTracerouteLoop() {
	ticker := time.NewTicker(TracerouteInterval)
	defer ticker.Stop()

	// Initial grace period before the first traceroute
	select {
	case <-s.ctx.Done():
		return
	case <-time.After(5 * time.Second):
		s.executeSerialTraces()
	case <-s.traceTrigger:
		s.executeSerialTraces()
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.executeSerialTraces()
		case <-s.traceTrigger:
			s.executeSerialTraces()
		}
	}
}

func (s *TargetScheduler) executeSerialTraces() {
	s.mu.Lock()
	targetsCopy := make([]protocol.TargetConfig, len(s.targets))
	copy(targetsCopy, s.targets)
	s.mu.Unlock()

	if len(targetsCopy) == 0 {
		return
	}

	log.Printf("[Traceroute] Starting 15-minute serial route trace across %d targets (max 30 hops)...\n", len(targetsCopy))

	// Strictly serial execution: one target after another, NO concurrency
	for i, t := range targetsCopy {
		if s.ctx.Err() != nil {
			return
		}

		log.Printf("[Traceroute] [%d/%d] Tracing route to %s (%s)...\n", i+1, len(targetsCopy), t.Name, t.Host)
		report := s.tracer.TraceTarget(s.ctx, t)
		s.client.SendTracerouteReport(report)

		// Brief delay between targets to prevent packet bursts
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}

	log.Printf("[Traceroute] 15-minute serial route trace round completed.\n")
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
