package parser

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type ParsedReceipt struct {
	Product string
	Price   float64
	OrderID string
	URL     string
	Source  string
	SKU     string
}

var (
	pricePattern = regexp.MustCompile(`(?i)(?:price|total|amount)\s*:?\s*\$?([0-9]+(?:\.[0-9]{1,2})?)`)
	orderPattern = regexp.MustCompile(`(?i)(?:order\s*id|order\s*number|order\s*#|confirmation\s*number)\s*:?\s*#?\s*([A-Za-z0-9-]+)`)
	urlPattern   = regexp.MustCompile(`https?://[^\s<>"']+`)
)

func ParseReceipt(text string) (ParsedReceipt, error) {
	lines := compactLines(text)
	if len(lines) == 0 {
		return ParsedReceipt{}, errors.New("receipt text is empty")
	}

	price, err := extractPrice(text)
	if err != nil {
		return ParsedReceipt{}, err
	}

	return ParsedReceipt{
		Product: extractProduct(lines),
		Price:   price,
		OrderID: extractOrderID(text),
		URL:     extractURL(text),
		Source:  "receipt",
	}, nil
}

func compactLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func extractPrice(text string) (float64, error) {
	match := pricePattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, errors.New("receipt price not found")
	}
	return strconv.ParseFloat(match[1], 64)
}

func extractProduct(lines []string) string {
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "order") || strings.Contains(lower, "price") || strings.Contains(lower, "total") {
			continue
		}
		return line
	}
	return lines[0]
}

func extractOrderID(text string) string {
	match := orderPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func extractURL(text string) string {
	match := urlPattern.FindString(text)
	return strings.TrimRight(match, ".,)")
}
