package config

import "testing"

func TestLoadDefaultsProtectLiveIntegrations(t *testing.T) {
	t.Setenv("RECLAIMO_API_ADDR", "")
	t.Setenv("RECLAIMO_NIMBLE_MODE", "")
	t.Setenv("CLICKHOUSE_ENABLED", "")

	cfg := Load()

	if cfg.Nimble.Mode != "mock" {
		t.Fatalf("Nimble mode = %q, want mock", cfg.Nimble.Mode)
	}
	if cfg.ClickHouse.Enabled {
		t.Fatal("ClickHouse should be disabled by default")
	}
	if !cfg.DemoEnabled {
		t.Fatal("demo mode should be enabled by default")
	}
}
