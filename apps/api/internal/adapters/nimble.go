package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

var moneyPattern = regexp.MustCompile(`\$?\s*([0-9][0-9,]*(?:\.[0-9]{2})?)`)

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

func (m *NimbleMonitor) FetchPrice(ctx context.Context, purchase domain.Purchase) (domain.PriceObservation, error) {
	if strings.TrimSpace(purchase.URL) == "" {
		return domain.PriceObservation{}, errors.New("purchase URL is required for Nimble live monitoring")
	}

	body, err := json.Marshal(nimbleExtractRequest{
		URL:     purchase.URL,
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.PriceObservation{}, fmt.Errorf("nimble extract returned %s", resp.Status)
	}

	var extracted nimbleExtractResponse
	if err := json.NewDecoder(resp.Body).Decode(&extracted); err != nil {
		return domain.PriceObservation{}, err
	}

	price, err := extractObservedPrice(extracted)
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

func extractObservedPrice(extracted nimbleExtractResponse) (float64, error) {
	for _, key := range []string{"price", "current_price", "sale_price"} {
		if value, ok := extracted.Data.Parsing[key]; ok {
			price, err := coercePrice(value)
			if err == nil {
				return price, nil
			}
		}
	}

	for _, text := range []string{extracted.Data.Markdown, extracted.Data.HTML} {
		price, err := parseFirstPrice(text)
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
		return parseFirstPrice(typed)
	default:
		return 0, fmt.Errorf("unsupported price type %T", value)
	}
}

func parseFirstPrice(text string) (float64, error) {
	match := moneyPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, errors.New("price not found")
	}
	return strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
}
