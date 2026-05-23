package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nydhy/reclaimo/apps/api/internal/adapters"
	"github.com/nydhy/reclaimo/apps/api/internal/config"
	"github.com/nydhy/reclaimo/apps/api/internal/domain"
	"github.com/nydhy/reclaimo/apps/api/internal/events"
	"github.com/nydhy/reclaimo/apps/api/internal/orchestrator"
)

type receiptRequest struct {
	Text string `json:"text"`
}

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventStore := buildEventStore(ctx, cfg)
	client := &http.Client{Timeout: 5 * time.Second}
	agent := orchestrator.New(
		eventStore,
		buildPriceMonitor(cfg, client),
		adapters.HTTPRecoveryPublisher{URL: cfg.RecoveryReportURL, Client: client},
		adapters.HTTPPaymentRail{URL: cfg.PaymentRailURL, Client: client},
		cfg.PollInterval,
		cfg.MaxChecksPerPurchase,
	)

	mux := http.NewServeMux()
	registerRoutes(mux, agent)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if cfg.DemoEnabled {
		go func() {
			time.Sleep(500 * time.Millisecond)
			agent.SeedDemo(ctx)
		}()
	}

	log.Printf("reclaimo api listening on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func buildEventStore(ctx context.Context, cfg config.Config) events.Store {
	store := events.NewMemoryStore()
	sinks := []events.Sink{}

	if cfg.ClickHouse.Enabled {
		sink := adapters.NewClickHouseSink(cfg.ClickHouse.Addr, cfg.ClickHouse.Database, cfg.ClickHouse.Username, cfg.ClickHouse.Password, http.DefaultClient)
		if err := sink.EnsureSchema(ctx); err != nil {
			log.Printf("clickhouse disabled after schema check failed: %v", err)
		} else {
			sinks = append(sinks, sink)
			log.Printf("clickhouse event sink enabled: database=%s", cfg.ClickHouse.Database)
		}
	}

	if cfg.Observability.Enabled {
		sinks = append(sinks, adapters.NewLogTelemetrySink(cfg.Observability.Service))
		log.Printf("observability event sink enabled: service=%s", cfg.Observability.Service)
	}

	if len(sinks) == 0 {
		return store
	}
	return events.NewMirrorStore(store, sinks...)
}

func buildPriceMonitor(cfg config.Config, client *http.Client) adapters.PriceMonitor {
	if cfg.Nimble.Mode != "live" {
		return adapters.NewMockPriceMonitor()
	}
	if cfg.Nimble.APIKey == "" {
		log.Println("nimble live mode requested without NIMBLE_API_KEY; using mock monitor")
		return adapters.NewMockPriceMonitor()
	}

	log.Printf("nimble live monitor enabled: base_url=%s driver=%s render=%t", cfg.Nimble.BaseURL, cfg.Nimble.Driver, cfg.Nimble.Render)
	return adapters.NewNimbleMonitor(cfg.Nimble.BaseURL, cfg.Nimble.APIKey, cfg.Nimble.Driver, cfg.Nimble.Render, client)
}

func registerRoutes(mux *http.ServeMux, agent *orchestrator.Orchestrator) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/receipts", func(w http.ResponseWriter, r *http.Request) {
		var req receiptRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if strings.TrimSpace(req.Text) == "" {
			writeError(w, http.StatusBadRequest, "receipt text is required")
			return
		}

		purchase, err := agent.IngestReceipt(r.Context(), req.Text)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]domain.Purchase{"purchase": purchase})
	})

	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			streamEvents(w, r, agent)
			return
		}
		writeJSON(w, http.StatusOK, map[string][]events.Event{"events": agent.ListEvents()})
	})

	mux.HandleFunc("GET /api/purchases", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string][]domain.PurchaseSnapshot{"purchases": agent.ListPurchases()})
	})

	mux.HandleFunc("GET /api/purchases/{id}", func(w http.ResponseWriter, r *http.Request) {
		snapshot, err := agent.PurchaseSnapshot(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]domain.PurchaseSnapshot{"purchase": snapshot})
	})

	mux.HandleFunc("POST /api/purchases/{id}/check", func(w http.ResponseWriter, r *http.Request) {
		snapshot, err := agent.CheckPurchaseByID(r.Context(), r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]domain.PurchaseSnapshot{"purchase": snapshot})
	})

	mux.HandleFunc("POST /api/reclaimo/recovery-report", func(w http.ResponseWriter, r *http.Request) {
		var report domain.RecoveryReport
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			writeError(w, http.StatusBadRequest, "invalid recovery report")
			return
		}
		log.Printf("recovery report accepted: product=%q recovery_amount=%.2f", report.Product, report.RecoveryAmount)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
	})

	mux.HandleFunc("POST /x402/transaction", func(w http.ResponseWriter, r *http.Request) {
		var intent domain.TransactionIntent
		if err := json.NewDecoder(r.Body).Decode(&intent); err != nil {
			writeError(w, http.StatusBadRequest, "invalid transaction intent")
			return
		}
		log.Printf("payment intent accepted: type=%q amount=%.2f", intent.Type, intent.Amount)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
	})
}

func streamEvents(w http.ResponseWriter, r *http.Request, agent *orchestrator.Orchestrator) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	for _, event := range agent.ListEvents() {
		writeSSE(w, event)
	}
	flusher.Flush()

	ch, cancel := agent.Subscribe()
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			writeSSE(w, event)
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event events.Event) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "event: %s\n", event.Type)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
