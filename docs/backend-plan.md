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

## Operational Notes

- Nimble trial calls are protected by `RECLAIMO_NIMBLE_MODE=mock` by default.
- Secrets belong in `.env` or a secret manager, never in committed files.
- Commit and push after each completed phase.

