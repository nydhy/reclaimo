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
	"github.com/nydhy/reclaimo/apps/api/internal/telemetry"
)

type Orchestrator struct {
	store     events.Store
	monitor   adapters.PriceMonitor
	publisher adapters.RecoveryPublisher
	payments  adapters.PaymentRail
	interval  time.Duration
	maxChecks int

	mu         sync.RWMutex
	emitMu     sync.Mutex
	purchases  map[string]*purchaseState
	idSequence int
}

type purchaseState struct {
	purchase       domain.Purchase
	status         domain.PurchaseStatus
	checkCount     int
	maxChecks      int
	lastCheckedAt  *time.Time
	lastObserved   *domain.PriceObservation
	recoveredAt    *time.Time
	terminalReason string
}

func New(store events.Store, monitor adapters.PriceMonitor, publisher adapters.RecoveryPublisher, payments adapters.PaymentRail, interval time.Duration, maxChecks int) *Orchestrator {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if maxChecks < 0 {
		maxChecks = 0
	}

	return &Orchestrator{
		store:     store,
		monitor:   monitor,
		publisher: publisher,
		payments:  payments,
		interval:  interval,
		maxChecks: maxChecks,
		purchases: make(map[string]*purchaseState),
	}
}

func (o *Orchestrator) IngestReceipt(ctx context.Context, text string) (purchase domain.Purchase, err error) {
	ctx, span := telemetry.StartSpan(ctx, "reclaimo.ingest_receipt", map[string]any{
		"component": "orchestrator",
		"source":    "receipt",
	})
	defer func() { span.Finish(err) }()

	parsed, err := parser.ParseReceipt(text)
	if err != nil {
		return domain.Purchase{}, err
	}

	purchase = domain.Purchase{
		ID:            o.nextID("purchase"),
		Product:       parsed.Product,
		BaselinePrice: parsed.Price,
		Source:        parsed.Source,
		OrderID:       parsed.OrderID,
		URL:           parsed.URL,
		CreatedAt:     time.Now().UTC(),
	}
	span.SetTag("purchase.id", purchase.ID)
	span.SetTag("purchase.product", purchase.Product)
	span.SetTag("purchase.baseline_price", purchase.BaselinePrice)
	span.SetTag("purchase.has_url", purchase.URL != "")

	o.mu.Lock()
	o.purchases[purchase.ID] = &purchaseState{
		purchase:  purchase,
		status:    domain.PurchaseStatusMonitoring,
		maxChecks: o.maxChecks,
	}
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
	o.checkPrice(ctx, purchase, true)
}

func (o *Orchestrator) CheckPurchaseByID(ctx context.Context, id string) (domain.PurchaseSnapshot, error) {
	purchase, err := o.purchaseByID(id)
	if err != nil {
		return domain.PurchaseSnapshot{}, err
	}

	o.checkPrice(ctx, purchase, true)
	return o.PurchaseSnapshot(id)
}

func (o *Orchestrator) ListPurchases() []domain.PurchaseSnapshot {
	o.mu.RLock()
	defer o.mu.RUnlock()

	out := make([]domain.PurchaseSnapshot, 0, len(o.purchases))
	for _, state := range o.purchases {
		out = append(out, snapshotFromState(state))
	}
	return out
}

func (o *Orchestrator) PurchaseSnapshot(id string) (domain.PurchaseSnapshot, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	state, ok := o.purchases[id]
	if !ok {
		return domain.PurchaseSnapshot{}, fmt.Errorf("purchase %s not found", id)
	}
	return snapshotFromState(state), nil
}

func (o *Orchestrator) monitorPurchase(ctx context.Context, purchase domain.Purchase) {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	if !o.checkPrice(ctx, purchase, false) {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !o.checkPrice(ctx, purchase, false) {
				return
			}
		}
	}
}

func (o *Orchestrator) checkPrice(ctx context.Context, purchase domain.Purchase, manual bool) bool {
	if !o.markCheckStarted(purchase.ID, manual) {
		return false
	}

	ctx, span := telemetry.StartSpan(ctx, "reclaimo.price_check", map[string]any{
		"component":               "monitor",
		"purchase.id":             purchase.ID,
		"purchase.product":        purchase.Product,
		"purchase.baseline_price": purchase.BaselinePrice,
		"manual":                  manual,
	})
	var spanErr error
	defer func() { span.Finish(spanErr) }()

	o.emit(events.PriceCheckStarted, map[string]any{
		"purchase_id": purchase.ID,
		"product":     purchase.Product,
		"manual":      manual,
	})

	observation, err := o.monitor.FetchPrice(ctx, purchase)
	if err != nil {
		spanErr = err
		o.emit(events.PriceUpdated, map[string]any{
			"purchase_id": purchase.ID,
			"error":       err.Error(),
		})
		return true
	}

	o.recordObservation(purchase.ID, observation)
	span.SetTag("price.current", observation.Price)
	span.SetTag("price.source", observation.Source)
	span.SetTag("price.available", observation.Available)
	span.SetTag("price.drop_detected", observation.Price < purchase.BaselinePrice)
	o.emit(events.PriceUpdated, map[string]any{"observation": observation})

	if observation.Price < purchase.BaselinePrice {
		o.handleDrop(ctx, purchase, observation)
	}

	return true
}

func (o *Orchestrator) handleDrop(ctx context.Context, purchase domain.Purchase, observation domain.PriceObservation) {
	ctx, span := telemetry.StartSpan(ctx, "reclaimo.recovery_workflow", map[string]any{
		"component":               "recovery",
		"purchase.id":             purchase.ID,
		"purchase.product":        purchase.Product,
		"purchase.baseline_price": purchase.BaselinePrice,
		"price.current":           observation.Price,
		"price.source":            observation.Source,
	})
	var spanErr error
	defer func() { span.Finish(spanErr) }()

	o.mu.Lock()
	state, ok := o.purchases[purchase.ID]
	if !ok {
		o.mu.Unlock()
		return
	}
	if state.status == domain.PurchaseStatusRecovered {
		o.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	state.status = domain.PurchaseStatusRecovered
	state.recoveredAt = &now
	state.terminalReason = "price_drop_recovered"
	o.mu.Unlock()

	recoveryAmount := purchase.BaselinePrice - observation.Price
	span.SetTag("recovery.amount", recoveryAmount)
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

	if err := o.publishRecoveryDossier(ctx, report); err == nil {
		o.emit(events.RecoveryPublished, map[string]any{"report": report})
	} else {
		spanErr = err
		o.emit(events.RecoveryPublished, map[string]any{"error": err.Error(), "report": report})
	}

	intent := domain.TransactionIntent{
		Type:      "price_recovery_claim",
		Amount:    recoveryAmount,
		Status:    "initiated",
		CreatedAt: time.Now().UTC(),
	}
	if err := o.triggerPaymentIntent(ctx, intent); err == nil {
		o.emit(events.PaymentTriggered, map[string]any{"transaction": intent})
	} else {
		if spanErr == nil {
			spanErr = err
		}
		o.emit(events.PaymentTriggered, map[string]any{"error": err.Error(), "transaction": intent})
	}
}

func (o *Orchestrator) publishRecoveryDossier(ctx context.Context, report domain.RecoveryReport) error {
	ctx, span := telemetry.StartSpan(ctx, "reclaimo.publish_recovery_dossier", map[string]any{
		"component":       "publisher",
		"product":         report.Product,
		"recovery.amount": report.RecoveryAmount,
	})
	err := o.publisher.Publish(ctx, report)
	span.Finish(err)
	return err
}

func (o *Orchestrator) triggerPaymentIntent(ctx context.Context, intent domain.TransactionIntent) error {
	ctx, span := telemetry.StartSpan(ctx, "reclaimo.trigger_payment_intent", map[string]any{
		"component": "payment_rail",
		"type":      intent.Type,
		"amount":    intent.Amount,
	})
	err := o.payments.Trigger(ctx, intent)
	span.Finish(err)
	return err
}

func (o *Orchestrator) markCheckStarted(id string, manual bool) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	state, ok := o.purchases[id]
	if !ok {
		return false
	}
	if state.status == domain.PurchaseStatusRecovered && !manual {
		return false
	}
	if state.status == domain.PurchaseStatusStopped && !manual {
		return false
	}
	if !manual && state.status == domain.PurchaseStatusMonitoring && state.maxChecks > 0 && state.checkCount >= state.maxChecks {
		state.status = domain.PurchaseStatusStopped
		state.terminalReason = "max_checks_reached"
		return false
	}

	now := time.Now().UTC()
	state.checkCount++
	state.lastCheckedAt = &now
	return true
}

func (o *Orchestrator) recordObservation(id string, observation domain.PriceObservation) {
	o.mu.Lock()
	defer o.mu.Unlock()

	state, ok := o.purchases[id]
	if !ok {
		return
	}
	state.lastObserved = &observation
}

func (o *Orchestrator) purchaseByID(id string) (domain.Purchase, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	state, ok := o.purchases[id]
	if !ok {
		return domain.Purchase{}, fmt.Errorf("purchase %s not found", id)
	}
	return state.purchase, nil
}

func snapshotFromState(state *purchaseState) domain.PurchaseSnapshot {
	return domain.PurchaseSnapshot{
		Purchase:       state.purchase,
		Status:         state.status,
		CheckCount:     state.checkCount,
		MaxChecks:      state.maxChecks,
		LastCheckedAt:  state.lastCheckedAt,
		LastObserved:   state.lastObserved,
		RecoveredAt:    state.recoveredAt,
		TerminalReason: state.terminalReason,
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
