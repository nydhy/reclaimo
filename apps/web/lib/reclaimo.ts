"use client";

export type EventType =
  | "PURCHASE_INGESTED"
  | "PRODUCT_EXTRACTED"
  | "PRICE_CHECK_STARTED"
  | "PRICE_UPDATED"
  | "PRICE_DROP_DETECTED"
  | "RECOVERY_REPORT_GENERATED"
  | "RECOVERY_PUBLISHED"
  | "PAYMENT_TRIGGERED";

export type AgentEvent = {
  id: string;
  type: EventType;
  version: string;
  timestamp: string;
  payload: Record<string, unknown>;
};

export type Purchase = {
  id: string;
  product: string;
  baseline_price: number;
  source: string;
  order_id?: string;
  url?: string;
  created_at: string;
};

export type PriceObservation = {
  purchase_id: string;
  product: string;
  price: number;
  url: string;
  available: boolean;
  timestamp: string;
};

export type PurchaseSnapshot = {
  purchase: Purchase;
  status: "monitoring" | "recovered" | "stopped";
  check_count: number;
  max_checks?: number;
  last_checked_at?: string;
  last_observed?: PriceObservation;
  recovered_at?: string;
  terminal_reason?: string;
};

export async function fetchPurchases(): Promise<PurchaseSnapshot[]> {
  const response = await fetch("/reclaimo-api/api/purchases", {
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error(`Unable to load purchases: ${response.status}`);
  }
  const body = (await response.json()) as { purchases: PurchaseSnapshot[] };
  return body.purchases;
}

export async function ingestReceipt(text: string): Promise<Purchase> {
  const response = await fetch("/reclaimo-api/api/receipts", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text }),
  });
  const body = await response.json();
  if (!response.ok) {
    throw new Error(body.error ?? `Receipt ingest failed: ${response.status}`);
  }
  return body.purchase as Purchase;
}

export async function runManualCheck(id: string): Promise<PurchaseSnapshot> {
  const response = await fetch(`/reclaimo-api/api/purchases/${id}/check`, {
    method: "POST",
  });
  const body = await response.json();
  if (!response.ok) {
    throw new Error(body.error ?? `Manual check failed: ${response.status}`);
  }
  return body.purchase as PurchaseSnapshot;
}

export function subscribeToEvents(onEvent: (event: AgentEvent) => void) {
  const source = new EventSource("/reclaimo-api/api/events");
  source.onmessage = (message) => {
    onEvent(JSON.parse(message.data) as AgentEvent);
  };
  source.addEventListener("PURCHASE_INGESTED", parseEvent(onEvent));
  source.addEventListener("PRODUCT_EXTRACTED", parseEvent(onEvent));
  source.addEventListener("PRICE_CHECK_STARTED", parseEvent(onEvent));
  source.addEventListener("PRICE_UPDATED", parseEvent(onEvent));
  source.addEventListener("PRICE_DROP_DETECTED", parseEvent(onEvent));
  source.addEventListener("RECOVERY_REPORT_GENERATED", parseEvent(onEvent));
  source.addEventListener("RECOVERY_PUBLISHED", parseEvent(onEvent));
  source.addEventListener("PAYMENT_TRIGGERED", parseEvent(onEvent));
  return () => source.close();
}

function parseEvent(onEvent: (event: AgentEvent) => void) {
  return (message: MessageEvent) => {
    onEvent(JSON.parse(message.data) as AgentEvent);
  };
}

export function money(value?: number) {
  if (typeof value !== "number") return "-";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(value);
}

export function compactTime(value?: string) {
  if (!value) return "-";
  return new Intl.DateTimeFormat("en-US", {
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(value));
}

