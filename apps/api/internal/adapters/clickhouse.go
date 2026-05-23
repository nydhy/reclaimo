package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nydhy/reclaimo/apps/api/internal/events"
)

type ClickHouseSink struct {
	addr     string
	database string
	username string
	password string
	client   *http.Client
}

func NewClickHouseSink(addr, database, username, password string, client *http.Client) *ClickHouseSink {
	if client == nil {
		client = http.DefaultClient
	}
	return &ClickHouseSink{
		addr:     strings.TrimRight(addr, "/"),
		database: database,
		username: username,
		password: password,
		client:   client,
	}
}

func (s *ClickHouseSink) EnsureSchema(ctx context.Context) error {
	if s.addr == "" {
		return errors.New("CLICKHOUSE_ADDR is required when CLICKHOUSE_ENABLED=true")
	}

	createDatabase := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", quoteIdentifier(s.database))
	if err := s.exec(ctx, createDatabase); err != nil {
		return err
	}

	createTable := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.events (
	id String,
	type String,
	version String,
	timestamp DateTime64(3, 'UTC'),
	payload String
)
ENGINE = MergeTree
ORDER BY (timestamp, type, id)
`, quoteIdentifier(s.database))

	return s.exec(ctx, createTable)
}

func (s *ClickHouseSink) Write(event events.Event) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}

	row, err := json.Marshal(map[string]any{
		"id":        event.ID,
		"type":      string(event.Type),
		"version":   event.Version,
		"timestamp": event.Timestamp.Format(time.RFC3339Nano),
		"payload":   string(payload),
	})
	if err != nil {
		return err
	}

	query := fmt.Sprintf("INSERT INTO %s.events FORMAT JSONEachRow", quoteIdentifier(s.database))
	return s.exec(context.Background(), query+"\n"+string(row))
}

func (s *ClickHouseSink) exec(ctx context.Context, query string) error {
	endpoint, err := url.Parse(s.addr)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewBufferString(query))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("clickhouse returned %s", resp.Status)
	}
	return nil
}

func quoteIdentifier(value string) string {
	if value == "" {
		value = "default"
	}

	cleaned := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		return -1
	}, value)
	if cleaned == "" {
		cleaned = "default"
	}
	return cleaned
}
