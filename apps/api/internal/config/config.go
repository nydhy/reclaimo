package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr              string
	DemoEnabled       bool
	PollInterval      time.Duration
	RecoveryReportURL string
	PaymentRailURL    string
	Nimble            NimbleConfig
	ClickHouse        ClickHouseConfig
	Observability     ObservabilityConfig
}

type NimbleConfig struct {
	Mode    string
	APIKey  string
	BaseURL string
	Driver  string
	Render  bool
}

type ClickHouseConfig struct {
	Enabled  bool
	Addr     string
	Database string
	Username string
	Password string
}

type ObservabilityConfig struct {
	Enabled bool
	Service string
}

func Load() Config {
	addr := env("RECLAIMO_API_ADDR", "127.0.0.1:8080")

	return Config{
		Addr:              addr,
		DemoEnabled:       envBool("RECLAIMO_DEMO_ENABLED", true),
		PollInterval:      envDuration("RECLAIMO_POLL_INTERVAL", 5*time.Second),
		RecoveryReportURL: env("RECOVERY_REPORT_URL", "http://"+addr+"/api/reclaimo/recovery-report"),
		PaymentRailURL:    env("PAYMENT_RAIL_URL", "http://"+addr+"/x402/transaction"),
		Nimble: NimbleConfig{
			Mode:    env("RECLAIMO_NIMBLE_MODE", "mock"),
			APIKey:  env("NIMBLE_API_KEY", ""),
			BaseURL: strings.TrimRight(env("NIMBLE_BASE_URL", "https://sdk.nimbleway.com/v1"), "/"),
			Driver:  env("NIMBLE_DRIVER", "vx6"),
			Render:  envBool("NIMBLE_RENDER", false),
		},
		ClickHouse: ClickHouseConfig{
			Enabled:  envBool("CLICKHOUSE_ENABLED", false),
			Addr:     strings.TrimRight(env("CLICKHOUSE_ADDR", ""), "/"),
			Database: env("CLICKHOUSE_DATABASE", "reclaimo"),
			Username: env("CLICKHOUSE_USERNAME", "default"),
			Password: env("CLICKHOUSE_PASSWORD", ""),
		},
		Observability: ObservabilityConfig{
			Enabled: envBool("DATADOG_ENABLED", false),
			Service: env("DD_SERVICE", "reclaimo-api"),
		},
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
