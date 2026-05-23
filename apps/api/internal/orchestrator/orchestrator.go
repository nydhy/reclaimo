package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nydhy/reclaimo/apps/api/internal/adapters"
	"github.com/nydhy/reclaimo/apps/api/internal/domain"
	"github.com/nydhy/reclaimo/apps/api/internal/events"
	"github.com/nydhy/reclaimo/apps/api/internal/parser"
)

type Orchestrator struct {
	store     events.Store
	monitor   adapters.PriceMonitor
	publisher adapters.RecoveryPublisher
	payments  adapters.PaymentRail
	interval  time.Duration

	mu         sync.RWMutex
	emitMu     sync.Mutex
	purchases  map[string]domain.Purchase
	recovered  map[string]bool
	idSequence int
}

func New(store events.Store, monitor adapters.PriceMonitor, publisher adapters.RecoveryPublisher, payments adapters.PaymentRail, interval time.Duration) *Orchestrator {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	return &Orchestrator{
		store:     store,
		monitor:   monitor,
		publisher: publisher,
		payments:  payments,
		interval:  interval,
		purchases: make(map[string]domain.Purchase),
		recovered: make(map[string]bool),
	}
}

func (o *Orchestrator) IngestReceipt(ctx context.Context, text string) (domain.Purchase, error) {
	parsed, err := parser.ParseReceipt(text)
	if err != nil {
		return domain.Purchase{}, err
	}

	purchase := domain.Purchase{
		ID:            o.nextID("purchase"),
		Product:       parsed.Product,
		BaselinePrice: parsed.Price,
		Source:        parsed.Source,
		OrderID:       parsed.OrderID,
		URL:           parsed.URL,
		CreatedAt:     time.Now().UTC(),
	}

	o.mu.Lock()
	o.purchases[purchase.ID] = purchase
	o.mu.Unlock()

	o.emit(events.PurchaseIngested, map[string]any{"purchase": purchase})
	o.emit(events.ProductExtracted, map[string]any{
		"purchase_id":    purchase.ID,
		"product":        purchase.Product,
		"baseline_price": purchase.BaselinePrice,
	})

	go o.monitorPurchase(ctx, purchase)

	return purchase, nil
}

func (o *Orchestrator) SeedDemo(ctx context.Context) {
	examples := []string{
		"Thanks for your order from Amazon\nMacBook Pro 14 M4\nPrice: $2199\nOrder ID: DEMO-1001",
		"Thanks for your order from Best Buy\nSony WH-1000XM5 Headphones\nPrice: $399\nOrder ID: DEMO-1002",
	}

	for _, example := range examples {
		_, _ = o.IngestReceipt(ctx, example)
	}
}

func (o *Orchestrator) ListEvents() []events.Event {
	return o.store.List()
}

func (o *Orchestrator) Subscribe() (<-chan events.Event, func()) {
	return o.store.Subscribe()
}

func (o *Orchestrator) CheckPurchase(ctx context.Context, purchase domain.Purchase) {
	o.checkPrice(ctx, purchase)
}

func (o *Orchestrator) monitorPurchase(ctx context.Context, purchase domain.Purchase) {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	o.checkPrice(ctx, purchase)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.checkPrice(ctx, purchase)
		}
	}
}

func (o *Orchestrator) checkPrice(ctx context.Context, purchase domain.Purchase) {
	o.emit(events.PriceCheckStarted, map[string]any{
		"purchase_id": purchase.ID,
		"product":     purchase.Product,
	})

	observation, err := o.monitor.FetchPrice(ctx, purchase)
	if err != nil {
		o.emit(events.PriceUpdated, map[string]any{
			"purchase_id": purchase.ID,
			"error":       err.Error(),
		})
		return
	}

	o.emit(events.PriceUpdated, map[string]any{"observation": observation})

	if observation.Price < purchase.BaselinePrice {
		o.handleDrop(ctx, purchase, observation)
	}
}

func (o *Orchestrator) handleDrop(ctx context.Context, purchase domain.Purchase, observation domain.PriceObservation) {
	o.mu.Lock()
	if o.recovered[purchase.ID] {
		o.mu.Unlock()
		return
	}
	o.recovered[purchase.ID] = true
	o.mu.Unlock()

	recoveryAmount := purchase.BaselinePrice - observation.Price
	o.emit(events.PriceDropDetected, map[string]any{
		"purchase_id":     purchase.ID,
		"baseline_price":  purchase.BaselinePrice,
		"current_price":   observation.Price,
		"recovery_amount": recoveryAmount,
	})

	report := domain.RecoveryReport{
		Product:        purchase.Product,
		BaselinePrice:  purchase.BaselinePrice,
		CurrentPrice:   observation.Price,
		RecoveryAmount: recoveryAmount,
		Status:         "triggered",
		GeneratedAt:    time.Now().UTC(),
	}
	o.emit(events.RecoveryReportGenerated, map[string]any{"report": report})

	if err := o.publisher.Publish(ctx, report); err == nil {
		o.emit(events.RecoveryPublished, map[string]any{"report": report})
	} else {
		o.emit(events.RecoveryPublished, map[string]any{"error": err.Error(), "report": report})
	}

	intent := domain.TransactionIntent{
		Type:      "price_recovery_claim",
		Amount:    recoveryAmount,
		Status:    "initiated",
		CreatedAt: time.Now().UTC(),
	}
	if err := o.payments.Trigger(ctx, intent); err == nil {
		o.emit(events.PaymentTriggered, map[string]any{"transaction": intent})
	} else {
		o.emit(events.PaymentTriggered, map[string]any{"error": err.Error(), "transaction": intent})
	}
}

func (o *Orchestrator) emit(eventType events.Type, payload map[string]any) {
	o.emitMu.Lock()
	defer o.emitMu.Unlock()

	o.store.Append(events.Event{
		ID:        o.nextID("event"),
		Type:      eventType,
		Version:   events.SchemaVersion,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	})
}

func (o *Orchestrator) nextID(prefix string) string {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.idSequence++
	return fmt.Sprintf("%s_%06d", prefix, o.idSequence)
}
