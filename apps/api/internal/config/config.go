package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                 string
	DemoEnabled          bool
	PollInterval         time.Duration
	MaxChecksPerPurchase int
	RecoveryReportURL    string
	PaymentRailURL       string
	AnthropicAPIKey      string
	Nimble               NimbleConfig
	ClickHouse           ClickHouseConfig
	Observability        ObservabilityConfig
	Email                EmailConfig
}

type NimbleConfig struct {
	Mode    string
	APIKey  string
	BaseURL string
	Driver  string
	Render  bool
	Timeout time.Duration
}

type ClickHouseConfig struct {
	Enabled  bool
	Addr     string
	Database string
	Username string
	Password string
}

type ObservabilityConfig struct {
	Enabled   bool
	Service   string
	AgentAddr string
	Env       string
}

type EmailConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       string
}

func Load() Config {
	addr := env("RECLAIMO_API_ADDR", "127.0.0.1:8080")

	return Config{
		Addr:                 addr,
		DemoEnabled:          envBool("RECLAIMO_DEMO_ENABLED", true),
		PollInterval:         envDuration("RECLAIMO_POLL_INTERVAL", 5*time.Second),
		MaxChecksPerPurchase: envInt("RECLAIMO_MAX_CHECKS_PER_PURCHASE", 0),
		RecoveryReportURL:    env("RECOVERY_REPORT_URL", "http://"+addr+"/api/reclaimo/recovery-report"),
		PaymentRailURL:       env("PAYMENT_RAIL_URL", "http://"+addr+"/x402/transaction"),
		AnthropicAPIKey:      env("ANTHROPIC_API_KEY", ""),
		Nimble: NimbleConfig{
			Mode:    env("RECLAIMO_NIMBLE_MODE", "mock"),
			APIKey:  env("NIMBLE_API_KEY", ""),
			BaseURL: strings.TrimRight(env("NIMBLE_BASE_URL", "https://sdk.nimbleway.com/v1"), "/"),
			Driver:  env("NIMBLE_DRIVER", "vx6"),
			Render:  envBool("NIMBLE_RENDER", false),
			Timeout: envDuration("NIMBLE_TIMEOUT", 30*time.Second),
		},
		ClickHouse: ClickHouseConfig{
			Enabled:  envBool("CLICKHOUSE_ENABLED", false),
			Addr:     strings.TrimRight(env("CLICKHOUSE_ADDR", ""), "/"),
			Database: env("CLICKHOUSE_DATABASE", "reclaimo"),
			Username: env("CLICKHOUSE_USERNAME", "default"),
			Password: env("CLICKHOUSE_PASSWORD", ""),
		},
		Observability: ObservabilityConfig{
			Enabled:   envBool("DATADOG_ENABLED", false),
			Service:   env("DD_SERVICE", "reclaimo-api"),
			AgentAddr: env("DD_AGENT_ADDR", "127.0.0.1:8126"),
			Env:       env("DD_ENV", "local"),
		},
		Email: EmailConfig{
			Enabled:  envBool("CLAIM_EMAIL_ENABLED", false),
			Host:     env("CLAIM_SMTP_HOST", ""),
			Port:     envInt("CLAIM_SMTP_PORT", 587),
			Username: env("CLAIM_SMTP_USERNAME", ""),
			Password: env("CLAIM_SMTP_PASSWORD", ""),
			From:     env("CLAIM_FROM_EMAIL", ""),
			To:       env("CLAIM_TO_EMAIL", ""),
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

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
