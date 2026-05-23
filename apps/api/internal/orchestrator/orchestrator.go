package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nydhy/reclaimo/apps/api/internal/adapters"
	"github.com/nydhy/reclaimo/apps/api/internal/domain"
	"github.com/nydhy/reclaimo/apps/api/internal/events"
	"github.com/nydhy/reclaimo/apps/api/internal/parser"
	"github.com/nydhy/reclaimo/apps/api/internal/telemetry"
)

type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

type EmailDrafter interface {
	DraftClaimEmail(ctx context.Context, product string, baselinePrice, currentPrice, recoveryAmount float64, orderID, retailer string, windowDays int) (string, error)
}

type Orchestrator struct {
	store    events.Store
	monitor  adapters.PriceMonitor
	publisher adapters.RecoveryPublisher
	payments  adapters.PaymentRail
	policy    *adapters.PolicyAnalyzer
	drafter   EmailDrafter
	email     EmailSender
	emailTo   string
	interval  time.Duration
	maxChecks int

	mu         sync.RWMutex
	emitMu     sync.Mutex
	purchases  map[string]*purchaseState
	orderIndex map[string]string
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
	policyAnalysis *domain.PolicyAnalysis
	claimPacket    *domain.ClaimPacket
	deadline       *time.Time
}

func New(store events.Store, monitor adapters.PriceMonitor, publisher adapters.RecoveryPublisher, payments adapters.PaymentRail, policy *adapters.PolicyAnalyzer, drafter EmailDrafter, email EmailSender, emailTo string, interval time.Duration, maxChecks int) *Orchestrator {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if maxChecks < 0 {
		maxChecks = 0
	}

	return &Orchestrator{
		store:      store,
		monitor:    monitor,
		publisher:  publisher,
		payments:   payments,
		policy:     policy,
		drafter:    drafter,
		email:      email,
		emailTo:    emailTo,
		interval:   interval,
		maxChecks:  maxChecks,
		purchases:  make(map[string]*purchaseState),
		orderIndex: make(map[string]string),
	}
}

func (o *Orchestrator) IngestReceipt(ctx context.Context, text string) (domain.Purchase, error) {
	parsed, err := parser.ParseReceipt(text)
	if err != nil {
		return domain.Purchase{}, err
	}
	return o.IngestParsed(ctx, parsed)
}

func (o *Orchestrator) IngestParsed(ctx context.Context, parsed parser.ParsedReceipt) (purchase domain.Purchase, err error) {
	ctx, span := telemetry.StartSpan(ctx, "reclaimo.ingest_receipt", map[string]any{
		"component": "orchestrator",
		"source":    parsed.Source,
	})
	defer func() { span.Finish(err) }()

	key := purchaseKey(parsed.Source, parsed.OrderID, parsed.URL)
	if key != "" {
		if existing, ok := o.purchaseByKey(key); ok {
			span.SetTag("purchase.duplicate", true)
			span.SetTag("purchase.id", existing.ID)
			return existing, nil
		}
	}

	purchase = domain.Purchase{
		ID:            o.nextID("purchase"),
		Product:       parsed.Product,
		BaselinePrice: parsed.Price,
		Source:        parsed.Source,
		OrderID:       parsed.OrderID,
		URL:           parsed.URL,
		SKU:           parsed.SKU,
		CreatedAt:     time.Now().UTC(),
	}

	o.mu.Lock()
	if key != "" {
		if existingID, ok := o.orderIndex[key]; ok {
			if existing, exists := o.purchases[existingID]; exists {
				o.mu.Unlock()
				span.SetTag("purchase.duplicate", true)
				span.SetTag("purchase.id", existing.purchase.ID)
				return existing.purchase, nil
			}
		}
	}
	o.purchases[purchase.ID] = &purchaseState{
		purchase:  purchase,
		status:    domain.PurchaseStatusMonitoring,
		maxChecks: o.maxChecks,
	}
	if key != "" {
		o.orderIndex[key] = purchase.ID
	}
	o.mu.Unlock()

	span.SetTag("purchase.id", purchase.ID)
	span.SetTag("purchase.product", purchase.Product)
	span.SetTag("purchase.baseline_price", purchase.BaselinePrice)
	span.SetTag("purchase.has_url", purchase.URL != "")
	span.SetTag("purchase.has_sku", purchase.SKU != "")

	o.emit(events.PurchaseIngested, map[string]any{"purchase": purchase})
	o.emit(events.ProductExtracted, map[string]any{
		"purchase_id":    purchase.ID,
		"product":        purchase.Product,
		"baseline_price": purchase.BaselinePrice,
	})

	go o.monitorPurchase(context.Background(), purchase)

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

func (o *Orchestrator) DeletePurchase(id string) error {
	o.mu.Lock()
	state, ok := o.purchases[id]
	if !ok {
		o.mu.Unlock()
		return fmt.Errorf("purchase %s not found", id)
	}
	delete(o.purchases, id)
	key := purchaseKey(state.purchase.Source, state.purchase.OrderID, state.purchase.URL)
	if key != "" {
		delete(o.orderIndex, key)
	}
	o.mu.Unlock()

	o.emit(events.PurchaseDeleted, map[string]any{
		"purchase_id": id,
		"product":     state.purchase.Product,
		"order_id":    state.purchase.OrderID,
	})
	return nil
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
		if !o.purchaseExists(purchase.ID) {
			return false
		}
		o.emit(events.PriceUpdated, map[string]any{
			"purchase_id": purchase.ID,
			"error":       err.Error(),
		})
		return true
	}

	if !o.recordObservation(purchase.ID, observation) {
		return false
	}
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

	go o.analyzePolicy(context.Background(), purchase, observation)
}

func (o *Orchestrator) analyzePolicy(ctx context.Context, purchase domain.Purchase, observation domain.PriceObservation) {
	ctx, span := telemetry.StartSpan(ctx, "reclaimo.analyze_policy", map[string]any{
		"component":   "policy_agent",
		"purchase.id": purchase.ID,
		"product":     purchase.Product,
	})
	defer span.Finish(nil)

	analysis := o.policy.Analyze(ctx, purchase)
	span.SetTag("policy.retailer", analysis.Retailer)
	span.SetTag("policy.eligible", analysis.Eligible)
	span.SetTag("policy.window_days", analysis.WindowDays)

	o.emit(events.PolicyFetched, map[string]any{"purchase_id": purchase.ID, "policy": analysis})
	o.emit(events.PolicyAnalyzed, map[string]any{"purchase_id": purchase.ID, "policy": analysis})

	if !analysis.Eligible {
		return
	}

	recoveryAmount := purchase.BaselinePrice - observation.Price
	draftBody, err := o.drafter.DraftClaimEmail(ctx, purchase.Product, purchase.BaselinePrice, observation.Price, recoveryAmount, purchase.OrderID, analysis.Retailer, analysis.WindowDays)
	if err != nil {
		draftBody = buildClaimEmailBody(purchase, observation, analysis, recoveryAmount)
	}

	packet := &domain.ClaimPacket{
		PurchaseID:     purchase.ID,
		Product:        purchase.Product,
		BaselinePrice:  purchase.BaselinePrice,
		CurrentPrice:   observation.Price,
		RecoveryAmount: recoveryAmount,
		OrderID:        purchase.OrderID,
		Policy:         analysis,
		DraftSubject:   fmt.Sprintf("Price Match Request – %s", purchase.Product),
		DraftBody:      draftBody,
		CreatedAt:      time.Now().UTC(),
	}

	deadline := purchase.CreatedAt.Add(time.Duration(analysis.WindowDays) * 24 * time.Hour)

	o.mu.Lock()
	state, ok := o.purchases[purchase.ID]
	if ok {
		state.policyAnalysis = &analysis
		state.claimPacket = packet
		state.status = domain.PurchaseStatusPendingClaim
		if state.deadline == nil && analysis.WindowDays > 0 {
			state.deadline = &deadline
		}
	}
	o.mu.Unlock()

	if ok {
		o.emit(events.ClaimPending, map[string]any{"purchase_id": purchase.ID, "claim": packet})
		go o.ApproveClaim(context.Background(), purchase.ID)
	}
}

func (o *Orchestrator) ApproveClaim(ctx context.Context, id string) error {
	o.mu.Lock()
	state, ok := o.purchases[id]
	if !ok {
		o.mu.Unlock()
		return fmt.Errorf("purchase %s not found", id)
	}
	if state.status != domain.PurchaseStatusPendingClaim {
		o.mu.Unlock()
		return fmt.Errorf("purchase %s is not pending claim (status: %s)", id, state.status)
	}
	if state.claimPacket == nil {
		o.mu.Unlock()
		return fmt.Errorf("purchase %s has no claim packet", id)
	}
	packet := *state.claimPacket
	o.mu.Unlock()

	ctx, span := telemetry.StartSpan(ctx, "reclaimo.approve_claim", map[string]any{
		"component":   "claim_agent",
		"purchase.id": id,
		"claim.to":    packet.Policy.ClaimEmail,
	})
	var spanErr error
	defer func() { span.Finish(spanErr) }()

	o.emit(events.ClaimApproved, map[string]any{"purchase_id": id, "claim": packet})

	to := packet.Policy.ClaimEmail
	if o.emailTo != "" {
		to = o.emailTo
	}

	if err := o.email.Send(ctx, to, packet.DraftSubject, packet.DraftBody); err != nil {
		spanErr = err
		o.emit(events.ClaimInitiated, map[string]any{"purchase_id": id, "error": err.Error()})
		return err
	}

	now := time.Now().UTC()
	o.mu.Lock()
	if s, ok := o.purchases[id]; ok {
		s.status = domain.PurchaseStatusClaimSubmitted
		if s.claimPacket != nil {
			s.claimPacket.SentAt = &now
		}
	}
	o.mu.Unlock()

	o.emit(events.ClaimInitiated, map[string]any{"purchase_id": id, "sent_to": to, "claim": packet})
	return nil
}

func buildClaimEmailBody(purchase domain.Purchase, observation domain.PriceObservation, policy domain.PolicyAnalysis, recoveryAmount float64) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Dear %s Customer Service,\n\n", policy.Retailer))
	sb.WriteString(fmt.Sprintf("I recently purchased %s and would like to request a price match under your %d-day price match guarantee.\n\n", purchase.Product, policy.WindowDays))
	sb.WriteString("Purchase details:\n")
	if purchase.OrderID != "" {
		sb.WriteString(fmt.Sprintf("  Order ID:       %s\n", purchase.OrderID))
	}
	sb.WriteString(fmt.Sprintf("  Product:        %s\n", purchase.Product))
	sb.WriteString(fmt.Sprintf("  Price paid:     $%.2f\n", purchase.BaselinePrice))
	sb.WriteString(fmt.Sprintf("  Current price:  $%.2f\n", observation.Price))
	sb.WriteString(fmt.Sprintf("  Refund amount:  $%.2f\n\n", recoveryAmount))
	sb.WriteString("Please process the price adjustment at your earliest convenience.\n\n")
	sb.WriteString("Thank you,\nReclaimo")
	return sb.String()
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
	if state.status == domain.PurchaseStatusPendingClaim && !manual {
		return false
	}
	if state.status == domain.PurchaseStatusClaimSubmitted && !manual {
		return false
	}
	if state.status == domain.PurchaseStatusStopped && !manual {
		return false
	}
	if !manual && state.deadline != nil && time.Now().UTC().After(*state.deadline) {
		state.status = domain.PurchaseStatusStopped
		state.terminalReason = "policy_window_expired"
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

func (o *Orchestrator) recordObservation(id string, observation domain.PriceObservation) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	state, ok := o.purchases[id]
	if !ok {
		return false
	}
	state.lastObserved = &observation
	return true
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

func (o *Orchestrator) purchaseByKey(key string) (domain.Purchase, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	id, ok := o.orderIndex[key]
	if !ok {
		return domain.Purchase{}, false
	}
	state, ok := o.purchases[id]
	if !ok {
		return domain.Purchase{}, false
	}
	return state.purchase, true
}

func (o *Orchestrator) purchaseExists(id string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()

	_, ok := o.purchases[id]
	return ok
}

func purchaseKey(source, orderID, url string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	orderID = strings.ToLower(strings.TrimSpace(orderID))
	if source != "" && orderID != "" {
		return source + ":order:" + orderID
	}

	url = strings.ToLower(strings.TrimSpace(url))
	if url != "" {
		return source + ":url:" + url
	}
	return ""
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
		PolicyAnalysis: state.policyAnalysis,
		ClaimPacket:    state.claimPacket,
		Deadline:       state.deadline,
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
