package ws

import (
	"testing"

	"github.com/guimc233/JustPing/host/internal/model"
)

func testTargets() []model.Target {
	return []model.Target{
		{ID: "t1", Name: "Alpha", Host: "1.1.1.1", PacketCount: 20, IntervalSec: 30},
		{ID: "t2", Name: "Beta", Host: "8.8.8.8", PacketCount: 10, IntervalSec: 60},
		{ID: "t3", Name: "Gamma", Host: "example.com", PacketCount: 5, IntervalSec: 15, DisableRoute: true},
	}
}

// Without overrides every enabled target is synced unchanged.
func TestBuildTargetConfigsNoOverrides(t *testing.T) {
	configs := buildTargetConfigsWithOverrides(testTargets(), "", "")

	if len(configs) != 3 {
		t.Fatalf("expected 3 configs, got %d", len(configs))
	}
	for _, c := range configs {
		// t3 disables route tracing on the target itself, so it stays disabled.
		wantDisabled := c.ID == "t3"
		if c.DisableRoute != wantDisabled {
			t.Errorf("target %s disable_route = %v, want %v", c.ID, c.DisableRoute, wantDisabled)
		}
	}
}

// A paused target is omitted from the sync, so the probe stops testing it.
func TestBuildTargetConfigsPausesTarget(t *testing.T) {
	configs := buildTargetConfigsWithOverrides(testTargets(), "", "t2")

	if len(configs) != 2 {
		t.Fatalf("expected the paused target to be omitted, got %d configs", len(configs))
	}
	for _, c := range configs {
		if c.ID == "t2" {
			t.Fatal("paused target t2 must not be synced")
		}
	}
}

// The pause list tolerates surrounding whitespace, blank entries and duplicates.
func TestBuildTargetConfigsParsesPauseListLoosely(t *testing.T) {
	configs := buildTargetConfigsWithOverrides(testTargets(), "", " t1 ,, t3 ,t1,")

	if len(configs) != 1 || configs[0].ID != "t2" {
		t.Fatalf("expected only t2 to survive, got %+v", configs)
	}
}

// A route override keeps the ICMP test and only turns route tracing off.
func TestBuildTargetConfigsRouteOverrideKeepsPing(t *testing.T) {
	configs := buildTargetConfigsWithOverrides(testTargets(), "t2", "")

	if len(configs) != 3 {
		t.Fatalf("expected route override to keep every target, got %d configs", len(configs))
	}
	for _, c := range configs {
		wantDisabled := c.ID == "t2" || c.ID == "t3" // t3 is disabled on the target itself
		if c.DisableRoute != wantDisabled {
			t.Errorf("target %s disable_route = %v, want %v", c.ID, c.DisableRoute, wantDisabled)
		}
		if c.Host == "" {
			t.Errorf("target %s lost its host", c.ID)
		}
	}
}

// A paused target is suppressed even when it also appears in the route list.
func TestBuildTargetConfigsPauseBeatsRouteOverride(t *testing.T) {
	configs := buildTargetConfigsWithOverrides(testTargets(), "t2", "t2")

	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
	for _, c := range configs {
		if c.ID == "t2" {
			t.Fatal("a paused target must not be synced even if route-disabled too")
		}
	}
}
