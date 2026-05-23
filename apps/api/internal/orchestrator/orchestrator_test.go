package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nydhy/reclaimo/apps/api/internal/domain"
	"github.com/nydhy/reclaimo/apps/api/internal/events"
)

type fakeMonitor struct {
	price float64
	err   error
}

func (m fakeMonitor) FetchPrice(_ context.Context, purchase domain.Purchase) (domain.PriceObservation, error) {
	if m.err != nil {
		return domain.PriceObservation{}, m.err
	}
	return domain.PriceObservation{
		PurchaseID: purchase.ID,
		Product:    purchase.Product,
		Price:      m.price,
		URL:        "https://example.com/product",
		Available:  true,
		Timestamp:  time.Now().UTC(),
	}, nil
}

type fakePublisher struct {
	calls int
	err   error
}

func (p *fakePublisher) Publish(context.Context, domain.RecoveryReport) error {
	p.calls++
	return p.err
}

type fakePaymentRail struct {
	calls int
	err   error
}

func (p *fakePaymentRail) Trigger(context.Context, domain.TransactionIntent) error {
	p.calls++
	return p.err
}

func TestCheckPurchaseTriggersRecoveryOnlyOnce(t *testing.T) {
	store := events.NewMemoryStore()
	publisher := &fakePublisher{}
	payments := &fakePaymentRail{}
	agent := New(store, fakeMonitor{price: 90}, publisher, payments, time.Hour)
	purchase := domain.Purchase{ID: "purchase_1", Product: "Test Product", BaselinePrice: 100}

	agent.CheckPurchase(context.Background(), purchase)
	agent.CheckPurchase(context.Background(), purchase)

	if publisher.calls != 1 {
		t.Fatalf("publisher calls = %d, want 1", publisher.calls)
	}
	if payments.calls != 1 {
		t.Fatalf("payment calls = %d, want 1", payments.calls)
	}

	assertEventCount(t, store.List(), events.PriceDropDetected, 1)
	assertEventCount(t, store.List(), events.RecoveryPublished, 1)
	assertEventCount(t, store.List(), events.PaymentTriggered, 1)
}

func TestCheckPurchaseRecordsRecoveryFailuresAsEvents(t *testing.T) {
	store := events.NewMemoryStore()
	agent := New(
		store,
		fakeMonitor{price: 90},
		&fakePublisher{err: errors.New("publisher unavailable")},
		&fakePaymentRail{err: errors.New("payment unavailable")},
		time.Hour,
	)

	agent.CheckPurchase(context.Background(), domain.Purchase{ID: "purchase_1", Product: "Test Product", BaselinePrice: 100})

	published := findLastEvent(t, store.List(), events.RecoveryPublished)
	if _, ok := published.Payload["error"]; !ok {
		t.Fatal("RECOVERY_PUBLISHED should include error payload")
	}

	payment := findLastEvent(t, store.List(), events.PaymentTriggered)
	if _, ok := payment.Payload["error"]; !ok {
		t.Fatal("PAYMENT_TRIGGERED should include error payload")
	}
}

func TestCheckPurchaseRecordsMonitorFailure(t *testing.T) {
	store := events.NewMemoryStore()
	agent := New(store, fakeMonitor{err: errors.New("monitor unavailable")}, &fakePublisher{}, &fakePaymentRail{}, time.Hour)

	agent.CheckPurchase(context.Background(), domain.Purchase{ID: "purchase_1", Product: "Test Product", BaselinePrice: 100})

	assertEventCount(t, store.List(), events.PriceCheckStarted, 1)
	updated := findLastEvent(t, store.List(), events.PriceUpdated)
	if _, ok := updated.Payload["error"]; !ok {
		t.Fatal("PRICE_UPDATED should include monitor error payload")
	}
}

func assertEventCount(t *testing.T, all []events.Event, eventType events.Type, want int) {
	t.Helper()

	got := 0
	for _, event := range all {
		if event.Type == eventType {
			got++
		}
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", eventType, got, want)
	}
}

func findLastEvent(t *testing.T, all []events.Event, eventType events.Type) events.Event {
	t.Helper()

	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Type == eventType {
			return all[i]
		}
	}
	t.Fatalf("event %s not found", eventType)
	return events.Event{}
}
