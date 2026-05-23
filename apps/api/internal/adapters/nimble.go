package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nydhy/reclaimo/apps/api/internal/domain"
)

type NimbleMonitor struct {
	baseURL string
	apiKey  string
	driver  string
	render  bool
	client  *http.Client
}

type nimbleExtractRequest struct {
	URL     string   `json:"url"`
	Render  bool     `json:"render"`
	Driver  string   `json:"driver,omitempty"`
	Formats []string `json:"formats,omitempty"`
}

type nimbleExtractResponse struct {
	URL        string `json:"url"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
	Data       struct {
		HTML     string         `json:"html"`
		Markdown string         `json:"markdown"`
		Parsing  map[string]any `json:"parsing"`
	} `json:"data"`
}

// Require $ to avoid matching plain numbers; decimal optional to handle split-span rendering.
var moneyPattern = regexp.MustCompile(`\$\s*([0-9][0-9,]*(?:\.[0-9]{2})?)`)

func NewNimbleMonitor(baseURL, apiKey, driver string, render bool, client *http.Client) *NimbleMonitor {
	if client == nil {
		client = http.DefaultClient
	}
	return &NimbleMonitor{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		driver:  driver,
		render:  render,
		client:  client,
	}
}

func constructSearchURL(purchase domain.Purchase) string {
	q := neturl.QueryEscape(purchase.Product)
	hay := strings.ToLower(purchase.Source + " " + purchase.URL)
	switch {
	case strings.Contains(hay, "bestbuy"):
		return "https://www.bestbuy.com/site/searchpage.jsp?st=" + q
	case strings.Contains(hay, "walmart"):
		return "https://www.walmart.com/search?q=" + q
	case strings.Contains(hay, "target"):
		return "https://www.target.com/s?searchTerm=" + q
	default:
		return "https://www.amazon.com/s?k=" + q
	}
}

func (m *NimbleMonitor) FetchPrice(ctx context.Context, purchase domain.Purchase) (domain.PriceObservation, error) {
	targetURL := strings.TrimSpace(purchase.URL)
	if targetURL == "" {
		targetURL = constructSearchURL(purchase)
	}

	body, err := json.Marshal(nimbleExtractRequest{
		URL:     targetURL,
		Render:  m.render,
		Driver:  m.driver,
		Formats: []string{"html", "markdown"},
	})
	if err != nil {
		return domain.PriceObservation{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/extract", bytes.NewReader(body))
	if err != nil {
		return domain.PriceObservation{}, err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return domain.PriceObservation{}, err
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	log.Printf("nimble url=%s status=%s body_len=%d markdown_snippet=%.500s", targetURL, resp.Status, len(rawBody), nimbleMarkdownSnippet(rawBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.PriceObservation{}, fmt.Errorf("nimble extract returned %s: %.200s", resp.Status, string(rawBody))
	}

	var extracted nimbleExtractResponse
	if err := json.Unmarshal(rawBody, &extracted); err != nil {
		return domain.PriceObservation{}, err
	}

	price, err := extractObservedPrice(extracted, purchase.BaselinePrice)
	if err != nil {
		return domain.PriceObservation{}, err
	}

	return domain.PriceObservation{
		PurchaseID: purchase.ID,
		Product:    purchase.Product,
		Price:      price,
		URL:        purchase.URL,
		Source:     "nimble",
		Available:  extracted.Status == "" || extracted.Status == "success",
		Timestamp:  time.Now().UTC(),
	}, nil
}

func nimbleMarkdownSnippet(rawBody []byte) string {
	var r nimbleExtractResponse
	if err := json.Unmarshal(rawBody, &r); err != nil {
		return string(rawBody[:min(len(rawBody), 200)])
	}
	if r.Data.Markdown != "" {
		return r.Data.Markdown
	}
	return r.Data.HTML[:min(len(r.Data.HTML), 500)]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func extractObservedPrice(extracted nimbleExtractResponse, baselineHint float64) (float64, error) {
	for _, key := range []string{"price", "current_price", "sale_price"} {
		if value, ok := extracted.Data.Parsing[key]; ok {
			price, err := coercePrice(value)
			if err == nil {
				return price, nil
			}
		}
	}

	for _, text := range []string{extracted.Data.Markdown, extracted.Data.HTML} {
		price, err := parseClosestPrice(text, baselineHint)
		if err == nil {
			return price, nil
		}
	}

	return 0, errors.New("price not found in Nimble response")
}

func coercePrice(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case string:
		return parseClosestPrice(typed, 0)
	default:
		return 0, fmt.Errorf("unsupported price type %T", value)
	}
}

// parseClosestPrice finds the price closest to baselineHint within ±60%.
// When baselineHint <= 0 it falls back to the most-frequent price heuristic,
// which works well on product detail pages.
func parseClosestPrice(text string, baselineHint float64) (float64, error) {
	matches := moneyPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return 0, errors.New("price not found")
	}

	var candidates []float64
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
		if err != nil || v < 1 {
			continue
		}
		candidates = append(candidates, float64(int(v*100+0.5))/100)
	}
	if len(candidates) == 0 {
		return 0, errors.New("price not found")
	}

	// With a baseline hint, pick the candidate closest to it within ±60%.
	if baselineHint > 0 {
		lo, hi := baselineHint*0.4, baselineHint*1.6
		var best float64
		bestDiff := -1.0
		for _, c := range candidates {
			if c < lo || c > hi {
				continue
			}
			diff := c - baselineHint
			if diff < 0 {
				diff = -diff
			}
			if bestDiff < 0 || diff < bestDiff {
				best = c
				bestDiff = diff
			}
		}
		if best > 0 {
			return best, nil
		}
	}

	// Fallback: most-frequent price (works well on product pages).
	freq := make(map[float64]int)
	for _, c := range candidates {
		if c >= 5 {
			freq[c]++
		}
	}
	var best float64
	var bestCount int
	for price, count := range freq {
		if count > bestCount || (count == bestCount && price < best) {
			best = price
			bestCount = count
		}
	}
	if best == 0 {
		return 0, errors.New("price not found")
	}
	return best, nil
}
