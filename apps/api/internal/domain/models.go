package domain

import "time"

type Purchase struct {
	ID            string    `json:"id"`
	Product       string    `json:"product"`
	BaselinePrice float64   `json:"baseline_price"`
	Source        string    `json:"source"`
	OrderID       string    `json:"order_id,omitempty"`
	URL           string    `json:"url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type PurchaseStatus string

const (
	PurchaseStatusMonitoring PurchaseStatus = "monitoring"
	PurchaseStatusRecovered  PurchaseStatus = "recovered"
	PurchaseStatusStopped    PurchaseStatus = "stopped"
)

type PurchaseSnapshot struct {
	Purchase       Purchase          `json:"purchase"`
	Status         PurchaseStatus    `json:"status"`
	CheckCount     int               `json:"check_count"`
	MaxChecks      int               `json:"max_checks,omitempty"`
	LastCheckedAt  *time.Time        `json:"last_checked_at,omitempty"`
	LastObserved   *PriceObservation `json:"last_observed,omitempty"`
	RecoveredAt    *time.Time        `json:"recovered_at,omitempty"`
	TerminalReason string            `json:"terminal_reason,omitempty"`
}

type PriceObservation struct {
	PurchaseID string    `json:"purchase_id"`
	Product    string    `json:"product"`
	Price      float64   `json:"price"`
	URL        string    `json:"url"`
	Available  bool      `json:"available"`
	Timestamp  time.Time `json:"timestamp"`
}

type RecoveryReport struct {
	Product        string    `json:"product"`
	BaselinePrice  float64   `json:"baseline_price"`
	CurrentPrice   float64   `json:"current_price"`
	RecoveryAmount float64   `json:"recovery_amount"`
	Status         string    `json:"status"`
	GeneratedAt    time.Time `json:"generated_at"`
}

type TransactionIntent struct {
	Type      string    `json:"type"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
