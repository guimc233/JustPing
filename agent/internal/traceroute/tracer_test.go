package traceroute

import (
	"context"
	"testing"
	"time"

	"github.com/guimc233/JustPing/shared/protocol"
)

func TestTracerouteStructure(t *testing.T) {
	tracer := NewTracer()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Invalid target test
	target := protocol.TargetConfig{
		ID:   "test-target-1",
		Host: "invalid.domain.test.local",
	}

	res := tracer.TraceTarget(ctx, target)
	if res.TargetID != target.ID {
		t.Errorf("expected target ID %s, got %s", target.ID, res.TargetID)
	}
	if len(res.Hops) > MaxHops {
		t.Errorf("hops exceeded max hops %d: got %d", MaxHops, len(res.Hops))
	}
}
