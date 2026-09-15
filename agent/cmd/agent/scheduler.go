package main

import (
	"context"
	"sync"
	"time"

	"github.com/guimc233/JustPing/agent/internal/client"
	"github.com/guimc233/JustPing/agent/internal/pinger"
	"github.com/guimc233/JustPing/shared/protocol"
)

type TargetScheduler struct {
	ctx     context.Context
	cancel  context.CancelFunc
	pinger  *pinger.Pinger
	client  *client.Client
	mu      sync.Mutex
	running map[string]context.CancelFunc
}

func NewTargetScheduler(parentCtx context.Context, p *pinger.Pinger, c *client.Client) *TargetScheduler {
	ctx, cancel := context.WithCancel(parentCtx)
	return &TargetScheduler{
		ctx:     ctx,
		cancel:  cancel,
		pinger:  p,
		client:  c,
		running: make(map[string]context.CancelFunc),
	}
}

func (s *TargetScheduler) SyncTargets(targets []protocol.TargetConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, stop := range s.running {
		stop()
	}
	s.running = make(map[string]context.CancelFunc)

	for _, t := range targets {
		targetCtx, targetCancel := context.WithCancel(s.ctx)
		s.running[t.ID] = targetCancel
		go s.runTargetLoop(targetCtx, t)
	}
}

func (s *TargetScheduler) runTargetLoop(ctx context.Context, t protocol.TargetConfig) {
	interval := time.Duration(t.IntervalSec) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second
	}

	// Immediate run upon sync
	res := s.pinger.PingTarget(ctx, t)
	s.client.QueueReport(protocol.PingReportPayload{Results: []protocol.SinglePingResult{res}})

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			res := s.pinger.PingTarget(ctx, t)
			s.client.QueueReport(protocol.PingReportPayload{Results: []protocol.SinglePingResult{res}})
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
