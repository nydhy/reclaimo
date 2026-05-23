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
- `POST /api/reclaimo/recovery-report`
- `POST /x402/transaction`

Demo mode is enabled by default and seeds two purchases. A forced price drop is emitted within 15 seconds without consuming Nimble trial quota.

## Configuration

Copy `.env.example` to `.env` for local development. Do not commit `.env`.

Real external calls are opt-in:

- Nimble calls require `RECLAIMO_NIMBLE_MODE=live`.
- Recovery publishing defaults to the local API endpoint.
- Payment rails are simulated until x402/CDP credentials exist.
