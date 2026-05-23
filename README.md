# Reclaimo

Reclaimo is an email-first autonomous price recovery agent. The backend ingests receipt text, extracts purchase data, monitors prices, emits a full event trace, publishes a recovery report, and triggers a payment intent.

## Current Scope

This repo is backend-first until the frontend design is ready.

- `apps/api` - Go API and agent orchestrator
- `infra` - local infrastructure notes
- `docs` - product and architecture notes

## Local Backend

```sh
cd apps/api
go run .
```

The API starts on `127.0.0.1:8080` by default.

- `GET /healthz`
- `POST /api/receipts`
- `GET /api/events`
- `GET /api/purchases`
- `GET /api/purchases/{id}`
- `POST /api/purchases/{id}/check`
- `POST /api/reclaimo/recovery-report`
- `POST /x402/transaction`

Demo mode is enabled by default and seeds two purchases. A forced price drop is emitted within 15 seconds without consuming Nimble trial quota.

## Configuration

Copy `.env.example` to `.env` for local development. Do not commit `.env`.

Real external calls are opt-in:

- Nimble calls require `RECLAIMO_NIMBLE_MODE=live`.
- Nimble live monitoring requires a product URL in the receipt text.
- Recovery publishing defaults to the local API endpoint.
- Payment rails are simulated until x402/CDP credentials exist.

## Live Integration Guardrails

By default, the backend uses deterministic mock pricing. To enable live Nimble extraction, set:

```sh
RECLAIMO_NIMBLE_MODE=live
NIMBLE_API_KEY=...
```

The live adapter uses Nimble's documented `POST /v1/extract` endpoint and Bearer authentication. Keep `RECLAIMO_POLL_INTERVAL` conservative while on a trial plan.

Use `RECLAIMO_MAX_CHECKS_PER_PURCHASE` to stop autonomous polling after a fixed number of checks per purchase. `0` means unlimited.

ClickHouse is optional:

```sh
CLICKHOUSE_ENABLED=true
CLICKHOUSE_ADDR=https://your-clickhouse-host:8443
CLICKHOUSE_USERNAME=default
CLICKHOUSE_PASSWORD=...
```

When enabled, events are mirrored to ClickHouse while the in-memory store remains active for local reads and SSE.

Verify ClickHouse writes without exposing secrets:

```sh
curl --user 'default:<password>' \
  --data-binary 'SELECT type, count() FROM reclaimo.events GROUP BY type ORDER BY type' \
  https://c6yash1lix.us-east-1.aws.clickhouse.cloud:8443
```
