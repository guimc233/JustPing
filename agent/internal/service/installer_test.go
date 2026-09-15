package service

import (
	"encoding/json"
	"testing"
)

func TestAgentConfigJSON(t *testing.T) {
	// Test default / omitted auto_update
	data := []byte(`{
		"server": "https://ping.example.com",
		"token": "secret-token",
		"china_mirror": true
	}`)

	var cfg AgentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cfg.Server != "https://ping.example.com" {
		t.Errorf("unexpected server: %s", cfg.Server)
	}
	if cfg.Token != "secret-token" {
		t.Errorf("unexpected token: %s", cfg.Token)
	}
	if !cfg.ChinaMirror {
		t.Errorf("expected china_mirror to be true")
	}
	if cfg.AutoUpdate != nil {
		t.Errorf("expected AutoUpdate to be nil when omitted")
	}
	if !cfg.IsAutoUpdateEnabled() {
		t.Errorf("expected IsAutoUpdateEnabled() to be true by default")
	}

	// Test explicitly disabled auto_update
	disabled := false
	cfg.AutoUpdate = &disabled
	marshaled, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var cfg2 AgentConfig
	if err := json.Unmarshal(marshaled, &cfg2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if cfg2.IsAutoUpdateEnabled() {
		t.Errorf("expected IsAutoUpdateEnabled() to be false")
	}
}
