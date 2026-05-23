package adapters

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/nydhy/reclaimo/apps/api/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNimbleMonitorRequiresPurchaseURL(t *testing.T) {
	monitor := NewNimbleMonitor("http://example.test", "key", "vx6", false, http.DefaultClient)

	_, err := monitor.FetchPrice(context.Background(), domain.Purchase{ID: "purchase_1"})
	if err == nil {
		t.Fatal("expected missing URL error")
	}
}

func TestNimbleMonitorExtractsPriceFromResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://sdk.test/extract" {
			t.Fatalf("url = %q, want https://sdk.test/extract", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q, want Bearer test-key", got)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"status":"success","data":{"markdown":"Current price is $1,999.00"}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	monitor := NewNimbleMonitor("https://sdk.test", "test-key", "vx6", false, client)

	observation, err := monitor.FetchPrice(context.Background(), domain.Purchase{
		ID:      "purchase_1",
		Product: "MacBook Pro 14 M4",
		URL:     "https://example.com/product",
	})
	if err != nil {
		t.Fatalf("FetchPrice returned error: %v", err)
	}

	if observation.Price != 1999 {
		t.Fatalf("price = %.2f, want 1999", observation.Price)
	}
}
