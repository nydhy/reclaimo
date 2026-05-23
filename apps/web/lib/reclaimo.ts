"use client";

export type EventType =
  | "PURCHASE_INGESTED"
  | "PURCHASE_DELETED"
  | "PRODUCT_EXTRACTED"
  | "PRICE_CHECK_STARTED"
  | "PRICE_UPDATED"
  | "PRICE_DROP_DETECTED"
  | "RECOVERY_REPORT_GENERATED"
  | "RECOVERY_PUBLISHED"
  | "PAYMENT_TRIGGERED"
  | "POLICY_FETCHED"
  | "POLICY_ANALYZED"
  | "CLAIM_PENDING"
  | "CLAIM_APPROVED"
  | "CLAIM_INITIATED";

export type AgentEvent = {
  id: string;
  type: EventType;
  version: string;
  timestamp: string;
  payload: Record<string, unknown>;
};

export type LapdogInfo = {
  running: boolean;
  version?: string;
  endpoints?: string[];
};

export type Purchase = {
  id: string;
  product: string;
  baseline_price: number;
  source: string;
  order_id?: string;
  url?: string;
  sku?: string;
  created_at: string;
};

export type PriceObservation = {
  purchase_id: string;
  product: string;
  price: number;
  url: string;
  source?: "demo" | "nimble" | "test" | string;
  available: boolean;
  timestamp: string;
};

export type PolicyAnalysis = {
  retailer: string;
  eligible: boolean;
  window_days: number;
  methods: string[];
  claim_email?: string;
  tat_days: string;
  policy_url?: string;
  fetched_at: string;
};

export type ClaimPacket = {
  purchase_id: string;
  product: string;
  baseline_price: number;
  current_price: number;
  recovery_amount: number;
  order_id?: string;
  policy: PolicyAnalysis;
  draft_subject: string;
  draft_body: string;
  sent_at?: string;
  created_at: string;
};

export type PurchaseSnapshot = {
  purchase: Purchase;
  status: "monitoring" | "recovered" | "pending_claim" | "claim_submitted" | "stopped";
  check_count: number;
  max_checks?: number;
  last_checked_at?: string;
  last_observed?: PriceObservation;
  recovered_at?: string;
  terminal_reason?: string;
  policy_analysis?: PolicyAnalysis;
  claim_packet?: ClaimPacket;
  deadline?: string;
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

export async function fetchEvents(): Promise<AgentEvent[]> {
  const response = await fetch("/reclaimo-api/api/events", {
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error(`Unable to load events: ${response.status}`);
  }
  const body = (await response.json()) as { events: AgentEvent[] };
  return body.events;
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

export async function uploadReceipt(file: File): Promise<Purchase> {
  const form = new FormData();
  form.append("receipt", file);
  const response = await fetch("/reclaimo-api/api/receipts/upload", {
    method: "POST",
    body: form,
  });
  const body = await response.json();
  if (!response.ok) {
    throw new Error(body.error ?? `Upload failed: ${response.status}`);
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

export async function approveClaim(id: string): Promise<void> {
  const response = await fetch(`/reclaimo-api/api/purchases/${id}/approve`, {
    method: "POST",
  });
  if (!response.ok) {
    const body = await response.json();
    throw new Error(body.error ?? `Approve failed: ${response.status}`);
  }
}

export async function deletePurchase(id: string): Promise<void> {
  const response = await fetch(`/reclaimo-api/api/purchases/${id}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    const body = await response.json();
    throw new Error(body.error ?? `Delete failed: ${response.status}`);
  }
}

export async function fetchLapdogInfo(): Promise<LapdogInfo> {
  const response = await fetch("/lapdog/info", { cache: "no-store" });
  if (!response.ok) {
    return { running: false };
  }
  const body = (await response.json()) as { version?: string; endpoints?: string[] };
  return { running: true, version: body.version, endpoints: body.endpoints ?? [] };
}

export function subscribeToEvents(
  onEvent: (event: AgentEvent) => void,
  onOpen?: () => void,
  onError?: () => void,
) {
  const source = new EventSource("/reclaimo-api/api/events");
  source.onopen = () => onOpen?.();
  source.onerror = () => onError?.();
  source.onmessage = (message) => {
    onEvent(JSON.parse(message.data) as AgentEvent);
  };
  source.addEventListener("PURCHASE_INGESTED", parseEvent(onEvent));
  source.addEventListener("PURCHASE_DELETED", parseEvent(onEvent));
  source.addEventListener("PRODUCT_EXTRACTED", parseEvent(onEvent));
  source.addEventListener("PRICE_CHECK_STARTED", parseEvent(onEvent));
  source.addEventListener("PRICE_UPDATED", parseEvent(onEvent));
  source.addEventListener("PRICE_DROP_DETECTED", parseEvent(onEvent));
  source.addEventListener("RECOVERY_REPORT_GENERATED", parseEvent(onEvent));
  source.addEventListener("RECOVERY_PUBLISHED", parseEvent(onEvent));
  source.addEventListener("PAYMENT_TRIGGERED", parseEvent(onEvent));
  source.addEventListener("POLICY_FETCHED", parseEvent(onEvent));
  source.addEventListener("POLICY_ANALYZED", parseEvent(onEvent));
  source.addEventListener("CLAIM_PENDING", parseEvent(onEvent));
  source.addEventListener("CLAIM_APPROVED", parseEvent(onEvent));
  source.addEventListener("CLAIM_INITIATED", parseEvent(onEvent));
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

export function eventSummary(event: AgentEvent) {
  const payload = event.payload;
  switch (event.type) {
    case "PURCHASE_INGESTED": {
      const purchase = payload.purchase as Purchase | undefined;
      return purchase
        ? `Registered ${purchase.product} at ${money(purchase.baseline_price)} from ${purchase.source}`
        : "Registered purchase";
    }
    case "PURCHASE_DELETED":
      return `Removed ${stringValue(payload.product)} from active monitoring`;
    case "PRODUCT_EXTRACTED":
      return `Extracted ${stringValue(payload.product)} with baseline ${money(numberValue(payload.baseline_price))}`;
    case "PRICE_CHECK_STARTED":
      return `${payload.manual ? "Manual" : "Autonomous"} price check started for ${stringValue(payload.product)}`;
    case "PRICE_UPDATED": {
      if (typeof payload.error === "string") return `Price check failed: ${payload.error}`;
      const observation = payload.observation as PriceObservation | undefined;
      return observation
        ? `${sourceName(observation.source)} observed ${money(observation.price)} for ${observation.product}`
        : "Price observation recorded";
    }
    case "PRICE_DROP_DETECTED":
      return `Detected ${money(numberValue(payload.recovery_amount))} recovery opportunity: ${money(
        numberValue(payload.baseline_price),
      )} to ${money(numberValue(payload.current_price))}`;
    case "RECOVERY_REPORT_GENERATED": {
      const report = payload.report as
        | { product?: string; recovery_amount?: number; current_price?: number }
        | undefined;
      return report
        ? `Generated recovery dossier for ${report.product ?? "purchase"} worth ${money(report.recovery_amount)}`
        : "Generated recovery dossier";
    }
    case "RECOVERY_PUBLISHED":
      return typeof payload.error === "string"
        ? `Dossier publish failed: ${payload.error}`
        : "Published recovery dossier to external endpoint";
    case "PAYMENT_TRIGGERED": {
      const transaction = payload.transaction as { amount?: number; status?: string } | undefined;
      if (typeof payload.error === "string") return `Payment rail failed: ${payload.error}`;
      return `Triggered payment intent for ${money(transaction?.amount)} (${transaction?.status ?? "initiated"})`;
    }
    case "POLICY_FETCHED":
    case "POLICY_ANALYZED": {
      const policy = payload.policy as PolicyAnalysis | undefined;
      if (!policy) return "Retailer policy analyzed";
      return policy.eligible
        ? `${policy.retailer} eligible — ${policy.window_days}-day window, ${policy.tat_days} TAT`
        : `${policy.retailer} policy: not eligible for price match`;
    }
    case "CLAIM_PENDING": {
      const claim = payload.claim as ClaimPacket | undefined;
      return claim
        ? `Claim ready for ${claim.product} — awaiting your approval to send email`
        : "Claim packet prepared, awaiting approval";
    }
    case "CLAIM_APPROVED":
      return `Claim approved — dispatching email to retailer`;
    case "CLAIM_INITIATED": {
      if (typeof payload.error === "string") return `Claim email failed: ${payload.error}`;
      return `Claim email sent to ${stringValue(payload.sent_to)}`;
    }
    default:
      return "Agent event recorded";
  }
}

function sourceName(source?: string) {
  if (source === "nimble") return "Live Nimble";
  if (source === "demo") return "Demo signal";
  return "Price signal";
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value : "purchase";
}

function numberValue(value: unknown) {
  return typeof value === "number" ? value : undefined;
}
