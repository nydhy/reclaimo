# Backend Implementation Plan

## Phase 0 - Foundation

Create the backend-first monorepo structure, root documentation, and environment contract.

## Phase 1 - Core Contracts

Define purchases, price observations, recovery reports, transaction intents, and versioned events.

## Phase 2 - Orchestrator

Ingest receipts, emit events, run monitoring checks, detect drops, and expose HTTP/SSE endpoints.

## Phase 3 - Recovery Workflow

Publish recovery reports and trigger simulated payment intents through adapter interfaces.

## Phase 4 - Real Integrations

Wire Nimble, ClickHouse, and Datadog/Lapdog behind explicit configuration flags.

Status:

- Nimble live adapter is available behind `RECLAIMO_NIMBLE_MODE=live`.
- ClickHouse event mirroring is available behind `CLICKHOUSE_ENABLED=true`.
- Lapdog/Datadog observability currently uses a log sink when `DATADOG_ENABLED=true`; run the API through Lapdog for local trace capture.

## Operational Notes

- Nimble trial calls are protected by `RECLAIMO_NIMBLE_MODE=mock` by default.
- Nimble live mode requires receipt text to include a product URL.
- `RECLAIMO_MAX_CHECKS_PER_PURCHASE` bounds autonomous monitoring loops for live-mode safety.
- Secrets belong in `.env` or a secret manager, never in committed files.
- Commit and push after each completed phase.
