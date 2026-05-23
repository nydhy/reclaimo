package events

import "time"

const SchemaVersion = "reclaimo.events.v1"

type Type string

const (
	PurchaseIngested        Type = "PURCHASE_INGESTED"
	ProductExtracted        Type = "PRODUCT_EXTRACTED"
	PriceCheckStarted       Type = "PRICE_CHECK_STARTED"
	PriceUpdated            Type = "PRICE_UPDATED"
	PriceDropDetected       Type = "PRICE_DROP_DETECTED"
	RecoveryReportGenerated Type = "RECOVERY_REPORT_GENERATED"
	RecoveryPublished       Type = "RECOVERY_PUBLISHED"
	PaymentTriggered        Type = "PAYMENT_TRIGGERED"
)

type Event struct {
	ID        string         `json:"id"`
	Type      Type           `json:"type"`
	Version   string         `json:"version"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload"`
}
