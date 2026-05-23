package adapters

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nydhy/reclaimo/apps/api/internal/parser"
)

type ClaudeEmailDrafter struct {
	apiKey string
	client *http.Client
}

func NewClaudeEmailDrafter(apiKey string) *ClaudeEmailDrafter {
	return &ClaudeEmailDrafter{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (d *ClaudeEmailDrafter) DraftClaimEmail(ctx context.Context, product string, baselinePrice, currentPrice, recoveryAmount float64, orderID, retailer string, windowDays int) (string, error) {
	orderRef := ""
	if orderID != "" {
		orderRef = "\nOrder ID: " + orderID
	}

	prompt := fmt.Sprintf(`Draft a price match refund request email on behalf of a customer. Write only the email body — no subject line, no greeting like "Dear...", start directly with the request. Be concise, polite, and specific. Include all dollar amounts.

Product: %s%s
Amount paid: $%.2f
Current listed price: $%.2f
Refund requested: $%.2f
Retailer: %s
Price match policy window: %d days`,
		product, orderRef, baselinePrice, currentPrice, recoveryAmount, retailer, windowDays)

	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload, _ := json.Marshal(map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 350,
		"messages":   []msg{{Role: "user", Content: prompt}},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", d.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("claude api returned %s", resp.Status)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("claude returned empty response")
	}
	return strings.TrimSpace(result.Content[0].Text), nil
}

// ParseReceiptImage uses Claude Vision to extract purchase details from a receipt image.
// mediaType must be image/jpeg, image/png, image/gif, or image/webp.
func (d *ClaudeEmailDrafter) ParseReceiptImage(ctx context.Context, data []byte, mediaType string) (parser.ParsedReceipt, error) {
	if d.apiKey == "" {
		return parser.ParsedReceipt{}, fmt.Errorf("claude API key not configured")
	}

	prompt := `Extract the single most expensive item from this receipt. Respond with a single JSON object only — no array, no markdown, no explanation:
{
  "product": "<full product name of the most expensive item>",
  "price": <price of that item as a number>,
  "source": "<retailer name, e.g. Amazon, Best Buy, Whole Foods>",
  "order_id": "<order or receipt number, or empty string>",
  "url": "<product URL if visible or empty string>",
  "sku": "<SKU, ASIN, model number, or empty string>"
}`

	// PDFs use the "document" block type; images use "image".
	var fileBlock map[string]any
	if mediaType == "application/pdf" {
		fileBlock = map[string]any{
			"type": "document",
			"source": map[string]any{
				"type":       "base64",
				"media_type": "application/pdf",
				"data":       base64.StdEncoding.EncodeToString(data),
			},
		}
	} else {
		fileBlock = map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": mediaType,
				"data":       base64.StdEncoding.EncodeToString(data),
			},
		}
	}

	payload, _ := json.Marshal(map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 256,
		"messages": []map[string]any{{
			"role":    "user",
			"content": []map[string]any{fileBlock, {"type": "text", "text": prompt}},
		}},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return parser.ParsedReceipt{}, err
	}
	req.Header.Set("x-api-key", d.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return parser.ParsedReceipt{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return parser.ParsedReceipt{}, fmt.Errorf("claude api returned %s", resp.Status)
	}

	var result struct {
		Content []struct{ Text string `json:"text"` } `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return parser.ParsedReceipt{}, err
	}
	if len(result.Content) == 0 {
		return parser.ParsedReceipt{}, fmt.Errorf("empty response from claude")
	}

	var parsed struct {
		Product string  `json:"product"`
		Price   float64 `json:"price"`
		Source  string  `json:"source"`
		OrderID string  `json:"order_id"`
		URL     string  `json:"url"`
		SKU     string  `json:"sku"`
	}
	text := strings.TrimSpace(result.Content[0].Text)
	// strip any accidental markdown fences
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		log.Printf("claude receipt parse error: response was: %s", text)
		return parser.ParsedReceipt{}, fmt.Errorf("failed to parse claude response: %w", err)
	}

	return parser.ParsedReceipt{
		Product: parsed.Product,
		Price:   parsed.Price,
		Source:  parsed.Source,
		OrderID: parsed.OrderID,
		URL:     parsed.URL,
		SKU:     parsed.SKU,
	}, nil
}

// NoopEmailDrafter returns an error so the orchestrator falls back to the built-in template.
type NoopEmailDrafter struct{}

func (NoopEmailDrafter) DraftClaimEmail(_ context.Context, _ string, _, _, _ float64, _, _ string, _ int) (string, error) {
	return "", fmt.Errorf("claude drafter not configured")
}
