package adapters

import (
	"context"
	"strings"
	"time"

	"github.com/nydhy/reclaimo/apps/api/internal/domain"
)

type PolicyAnalyzer struct{}

func NewPolicyAnalyzer() *PolicyAnalyzer {
	return &PolicyAnalyzer{}
}

var knownPolicies = map[string]domain.PolicyAnalysis{
	"amazon": {
		Retailer:   "Amazon",
		Eligible:   true,
		WindowDays: 30,
		Methods:    []string{"email"},
		ClaimEmail: "cs-reply@amazon.com",
		TATDays:    "2–3 business days",
		PolicyURL:  "https://www.amazon.com/gp/help/customer/display.html?nodeId=GKM69DUUYKQWKWX7",
	},
	"bestbuy": {
		Retailer:   "Best Buy",
		Eligible:   true,
		WindowDays: 15,
		Methods:    []string{"email"},
		ClaimEmail: "BestBuyInfo@emailinfo.bestbuy.com",
		TATDays:    "3–5 business days",
		PolicyURL:  "https://www.bestbuy.com/site/help-topics/best-buy-price-match-guarantee/pcmcat290300050002.c",
	},
	"target": {
		Retailer:   "Target",
		Eligible:   true,
		WindowDays: 14,
		Methods:    []string{"email"},
		ClaimEmail: "target.guestrelations@target.com",
		TATDays:    "3–5 business days",
	},
	"walmart": {
		Retailer:   "Walmart",
		Eligible:   true,
		WindowDays: 7,
		Methods:    []string{"email"},
		ClaimEmail: "help@walmart.com",
		TATDays:    "5–7 business days",
	},
}

func (a *PolicyAnalyzer) Analyze(_ context.Context, purchase domain.Purchase) domain.PolicyAnalysis {
	retailer := detectRetailer(purchase)
	policy, ok := knownPolicies[retailer]
	if !ok {
		policy = domain.PolicyAnalysis{
			Retailer:   strings.Title(retailer),
			Eligible:   false,
			WindowDays: 0,
			Methods:    []string{"email"},
			TATDays:    "varies by retailer",
		}
	}
	policy.FetchedAt = time.Now().UTC()
	return policy
}

func detectRetailer(purchase domain.Purchase) string {
	candidates := []string{"amazon", "bestbuy", "target", "walmart"}
	haystack := strings.ToLower(purchase.URL + " " + purchase.Source + " " + purchase.Product)
	for _, name := range candidates {
		if strings.Contains(haystack, name) {
			return name
		}
	}
	return "unknown"
}
