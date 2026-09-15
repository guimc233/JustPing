package pinger

import (
	"context"
	"testing"
	"time"

	"github.com/guimc233/JustPing/shared/protocol"
)

func TestTargetWindowAggregation(t *testing.T) {
	win := NewTargetWindow(5)

	// Empty window
	emptySum := win.Summary()
	if emptySum.TotalSent != 0 {
		t.Errorf("expected 0 total sent, got %d", emptySum.TotalSent)
	}

	// 1 success sample
	win.Add(SinglePingStat{
		Timestamp: time.Now(),
		Success:   true,
		RTT:       20.5,
	})

	sum1 := win.Summary()
	if sum1.TotalSent != 1 || sum1.TotalRecv != 1 {
		t.Fatalf("expected 1 sent / 1 recv, got %d/%d", sum1.TotalSent, sum1.TotalRecv)
	}
	if sum1.LossPct != 0.0 {
		t.Errorf("expected 0%% loss, got %f", sum1.LossPct)
	}
	if sum1.AvgRTT != 20.5 || sum1.MinRTT != 20.5 || sum1.MaxRTT != 20.5 {
		t.Errorf("unexpected RTTs: avg=%f min=%f max=%f", sum1.AvgRTT, sum1.MinRTT, sum1.MaxRTT)
	}

	// 1 failure sample
	win.Add(SinglePingStat{
		Timestamp: time.Now(),
		Success:   false,
		ErrorMsg:  "request timeout",
	})

	sum2 := win.Summary()
	if sum2.TotalSent != 2 || sum2.TotalRecv != 1 {
		t.Fatalf("expected 2 sent / 1 recv, got %d/%d", sum2.TotalSent, sum2.TotalRecv)
	}
	if sum2.LossPct != 50.0 {
		t.Errorf("expected 50%% loss, got %f", sum2.LossPct)
	}

	// Add more successes to slide window beyond maxKeep (5)
	win.Add(SinglePingStat{Timestamp: time.Now(), Success: true, RTT: 30.0})
	win.Add(SinglePingStat{Timestamp: time.Now(), Success: true, RTT: 25.0})
	win.Add(SinglePingStat{Timestamp: time.Now(), Success: true, RTT: 15.0})
	win.Add(SinglePingStat{Timestamp: time.Now(), Success: true, RTT: 10.0})

	sum3 := win.Summary()
	if sum3.TotalSent != 5 {
		t.Errorf("expected window capped at 5, got %d", sum3.TotalSent)
	}
	// The first success (20.5) has fallen off, elements are: fail, 30.0, 25.0, 15.0, 10.0
	if sum3.TotalRecv != 4 {
		t.Errorf("expected 4 recv, got %d", sum3.TotalRecv)
	}
	if sum3.LossPct != 20.0 {
		t.Errorf("expected 20%% loss (1/5), got %f", sum3.LossPct)
	}
	if sum3.MinRTT != 10.0 || sum3.MaxRTT != 30.0 {
		t.Errorf("expected min 10.0 and max 30.0, got min=%f max=%f", sum3.MinRTT, sum3.MaxRTT)
	}
}

func TestPingTargetOnceWindowIntegration(t *testing.T) {
	p := NewPinger()
	target := protocol.TargetConfig{
		ID:          "target-test-1",
		Host:        "invalid.domain.example.invalid",
		PacketCount: 10,
		IntervalSec: 30,
	}

	res := p.PingTargetOnce(context.Background(), target)
	if res.TargetID != target.ID {
		t.Errorf("expected target ID %s, got %s", target.ID, res.TargetID)
	}
	if res.PacketsSent != 1 {
		t.Errorf("expected 1 packet sent in window, got %d", res.PacketsSent)
	}
	if res.LossPct != 100.0 {
		t.Errorf("expected 100%% loss on invalid host, got %f", res.LossPct)
	}
}
