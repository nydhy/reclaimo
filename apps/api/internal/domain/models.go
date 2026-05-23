package domain

import "time"

type Purchase struct {
	ID            string    `json:"id"`
	Product       string    `json:"product"`
	BaselinePrice float64   `json:"baseline_price"`
	Source        string    `json:"source"`
	OrderID       string    `json:"order_id,omitempty"`
	URL           string    `json:"url,omitempty"`
	SKU           string    `json:"sku,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type PurchaseStatus string

const (
	PurchaseStatusMonitoring    PurchaseStatus = "monitoring"
	PurchaseStatusRecovered     PurchaseStatus = "recovered"
	PurchaseStatusPendingClaim   PurchaseStatus = "pending_claim"
	PurchaseStatusClaimSubmitted PurchaseStatus = "claim_submitted"
	PurchaseStatusStopped       PurchaseStatus = "stopped"
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
	PolicyAnalysis *PolicyAnalysis   `json:"policy_analysis,omitempty"`
	ClaimPacket    *ClaimPacket      `json:"claim_packet,omitempty"`
	Deadline       *time.Time        `json:"deadline,omitempty"`
}

type PolicyAnalysis struct {
	Retailer   string    `json:"retailer"`
	Eligible   bool      `json:"eligible"`
	WindowDays int       `json:"window_days"`
	Methods    []string  `json:"methods"`
	ClaimEmail string    `json:"claim_email,omitempty"`
	TATDays    string    `json:"tat_days"`
	PolicyURL  string    `json:"policy_url,omitempty"`
	FetchedAt  time.Time `json:"fetched_at"`
}

type ClaimPacket struct {
	PurchaseID     string         `json:"purchase_id"`
	Product        string         `json:"product"`
	BaselinePrice  float64        `json:"baseline_price"`
	CurrentPrice   float64        `json:"current_price"`
	RecoveryAmount float64        `json:"recovery_amount"`
	OrderID        string         `json:"order_id,omitempty"`
	Policy         PolicyAnalysis `json:"policy"`
	DraftSubject   string         `json:"draft_subject"`
	DraftBody      string         `json:"draft_body"`
	SentAt         *time.Time     `json:"sent_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type PriceObservation struct {
	PurchaseID string    `json:"purchase_id"`
	Product    string    `json:"product"`
	Price      float64   `json:"price"`
	URL        string    `json:"url"`
	Source     string    `json:"source"`
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
