package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nydhy/reclaimo/apps/api/internal/domain"
)

type PriceMonitor interface {
	FetchPrice(ctx context.Context, purchase domain.Purchase) (domain.PriceObservation, error)
}

type RecoveryPublisher interface {
	Publish(ctx context.Context, report domain.RecoveryReport) error
}

type PaymentRail interface {
	Trigger(ctx context.Context, intent domain.TransactionIntent) error
}

type MockPriceMonitor struct {
	mu   sync.Mutex
	seen map[string]int
}

func NewMockPriceMonitor() *MockPriceMonitor {
	return &MockPriceMonitor{seen: make(map[string]int)}
}

func (m *MockPriceMonitor) FetchPrice(_ context.Context, purchase domain.Purchase) (domain.PriceObservation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.seen[purchase.ID]++

	price := purchase.BaselinePrice
	if strings.Contains(strings.ToLower(purchase.Product), "macbook") && m.seen[purchase.ID] >= 2 {
		price = purchase.BaselinePrice - 200
	}
	if strings.Contains(strings.ToLower(purchase.Product), "sony") && m.seen[purchase.ID] >= 3 {
		price = purchase.BaselinePrice - 35
	}

	return domain.PriceObservation{
		PurchaseID: purchase.ID,
		Product:    purchase.Product,
		Price:      price,
		URL:        "https://example.com/reclaimo/mock-price",
		Source:     "demo",
		Available:  true,
		Timestamp:  time.Now().UTC(),
	}, nil
}

type HTTPRecoveryPublisher struct {
	URL    string
	Client *http.Client
}

func (p HTTPRecoveryPublisher) Publish(ctx context.Context, report domain.RecoveryReport) error {
	return postJSON(ctx, p.Client, p.URL, report)
}

type HTTPPaymentRail struct {
	URL    string
	Client *http.Client
}

func (p HTTPPaymentRail) Trigger(ctx context.Context, intent domain.TransactionIntent) error {
	return postJSON(ctx, p.Client, p.URL, intent)
}

func postJSON(ctx context.Context, client *http.Client, url string, payload any) error {
	if client == nil {
		client = http.DefaultClient
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("post %s returned %s", url, resp.Status)
	}
	return nil
}
