package parser

import "testing"

func TestParseReceiptExtractsProductPriceAndOrderID(t *testing.T) {
	receipt := "Thanks for your order from Amazon\nMacBook Pro 14 M4\nPrice: $2199\nOrder ID: 12345"

	parsed, err := ParseReceipt(receipt)
	if err != nil {
		t.Fatalf("ParseReceipt returned error: %v", err)
	}

	if parsed.Product != "MacBook Pro 14 M4" {
		t.Fatalf("product = %q, want MacBook Pro 14 M4", parsed.Product)
	}
	if parsed.Price != 2199 {
		t.Fatalf("price = %.2f, want 2199", parsed.Price)
	}
	if parsed.OrderID != "12345" {
		t.Fatalf("order id = %q, want 12345", parsed.OrderID)
	}
}

func TestParseReceiptDoesNotTreatOrderFromAsOrderID(t *testing.T) {
	receipt := "Thanks for your order from Amazon\nMacBook Pro 14 M4\nPrice: $2199"

	parsed, err := ParseReceipt(receipt)
	if err != nil {
		t.Fatalf("ParseReceipt returned error: %v", err)
	}

	if parsed.OrderID != "" {
		t.Fatalf("order id = %q, want empty", parsed.OrderID)
	}
}
