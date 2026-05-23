package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nydhy/reclaimo/apps/api/internal/adapters"
	"github.com/nydhy/reclaimo/apps/api/internal/events"
	"github.com/nydhy/reclaimo/apps/api/internal/orchestrator"
)

func TestPostReceiptsCreatesPurchase(t *testing.T) {
	mux := testMux()
	body := `{"text":"Thanks for your order\nMacBook Pro 14 M4\nPrice: $2199\nOrder ID: TEST-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/receipts", strings.NewReader(body))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var response struct {
		Purchase struct {
			Product       string  `json:"product"`
			BaselinePrice float64 `json:"baseline_price"`
			OrderID       string  `json:"order_id"`
		} `json:"purchase"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Purchase.Product != "MacBook Pro 14 M4" {
		t.Fatalf("product = %q, want MacBook Pro 14 M4", response.Purchase.Product)
	}
	if response.Purchase.BaselinePrice != 2199 {
		t.Fatalf("baseline price = %.2f, want 2199", response.Purchase.BaselinePrice)
	}
	if response.Purchase.OrderID != "TEST-1" {
		t.Fatalf("order id = %q, want TEST-1", response.Purchase.OrderID)
	}
}

func TestPostReceiptsValidatesBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "invalid json", body: `{`, want: http.StatusBadRequest},
		{name: "empty text", body: `{"text":"   "}`, want: http.StatusBadRequest},
		{name: "missing price", body: `{"text":"MacBook Pro 14 M4"}`, want: http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/receipts", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			testMux().ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}

func TestGetEventsReturnsEvents(t *testing.T) {
	store := events.NewMemoryStore()
	agent := orchestrator.New(
		store,
		adapters.NewMockPriceMonitor(),
		adapters.HTTPRecoveryPublisher{URL: "http://127.0.0.1:1"},
		adapters.HTTPPaymentRail{URL: "http://127.0.0.1:1"},
		time.Hour,
	)
	_, err := agent.IngestReceipt(context.Background(), "MacBook Pro 14 M4\nPrice: $2199")
	if err != nil {
		t.Fatalf("IngestReceipt returned error: %v", err)
	}

	mux := http.NewServeMux()
	registerRoutes(mux, agent)
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), string(events.PurchaseIngested)) {
		t.Fatalf("events response does not include %s: %s", events.PurchaseIngested, rec.Body.String())
	}
}

func testMux() *http.ServeMux {
	store := events.NewMemoryStore()
	agent := orchestrator.New(
		store,
		adapters.NewMockPriceMonitor(),
		adapters.HTTPRecoveryPublisher{URL: "http://127.0.0.1:1"},
		adapters.HTTPPaymentRail{URL: "http://127.0.0.1:1"},
		time.Hour,
	)
	mux := http.NewServeMux()
	registerRoutes(mux, agent)
	return mux
}
