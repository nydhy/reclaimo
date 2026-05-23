"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AgentEvent,
  approveClaim,
  deletePurchase,
  EventType,
  fetchEvents,
  fetchLapdogInfo,
  fetchPurchases,
  ingestReceipt,
  LapdogInfo,
  money,
  PriceObservation,
  PurchaseSnapshot,
  runManualCheck,
  subscribeToEvents,
  uploadReceipt,
} from "./reclaimo";

export type AgentStatusLevel = "idle" | "running" | "waiting" | "done" | "error";
export type AgentStatus = { level: AgentStatusLevel; detail: string };
export type AgentStatuses = {
  monitor: AgentStatus;
  recovery: AgentStatus;
  policy: AgentStatus;
  claim: AgentStatus;
};

const eventOrder = [
  "PURCHASE_INGESTED",
  "PRODUCT_EXTRACTED",
  "PRICE_CHECK_STARTED",
  "PRICE_UPDATED",
  "PRICE_DROP_DETECTED",
  "RECOVERY_REPORT_GENERATED",
  "RECOVERY_PUBLISHED",
  "PAYMENT_TRIGGERED",
];

export function useReclaimo() {
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [purchases, setPurchases] = useState<PurchaseSnapshot[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [connected, setConnected] = useState(false);
  const [lapdog, setLapdog] = useState<LapdogInfo>({ running: false });
  const [justRecovered, setJustRecovered] = useState(false);
  const lastDropIdRef = useRef<string | null>(null);

  const refreshPurchases = useCallback(async () => {
    try {
      setPurchases(sortPurchases(await fetchPurchases()));
      setConnected(true);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load purchases");
      setConnected(false);
    }
  }, []);

  const refreshEvents = useCallback(async () => {
    try {
      const loaded = await fetchEvents();
      setEvents((current) => dedupeEvents([...current, ...loaded]).slice(-160));
      setConnected(true);
    } catch {
      // SSE may still be healthy; avoid replacing a useful API error from purchases.
    }
  }, []);

  const refreshLapdog = useCallback(async () => {
    setLapdog(await fetchLapdogInfo());
  }, []);

  useEffect(() => {
    refreshPurchases();
    refreshEvents();
    refreshLapdog();
    const interval = window.setInterval(() => {
      refreshPurchases();
      refreshEvents();
      refreshLapdog();
    }, 2500);
    return () => window.clearInterval(interval);
  }, [refreshEvents, refreshLapdog, refreshPurchases]);

  useEffect(() => {
    const close = subscribeToEvents(
      (event) => {
        setConnected(true);
        setEvents((current) => dedupeEvents([...current, event]).slice(-160));
        if (
          event.type === "PURCHASE_INGESTED" ||
          event.type === "PURCHASE_DELETED" ||
          event.type === "PRICE_UPDATED" ||
          event.type === "PRICE_DROP_DETECTED" ||
          event.type === "PAYMENT_TRIGGERED" ||
          event.type === "CLAIM_PENDING" ||
          event.type === "CLAIM_INITIATED"
        ) {
          refreshPurchases();
        }
      },
      () => setConnected(true),
      () => refreshEvents(),
    );
    return close;
  }, [refreshEvents, refreshPurchases]);

  const submitReceipt = useCallback(
    async (text: string) => {
      setBusy(true);
      try {
        await ingestReceipt(text);
        await refreshPurchases();
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Receipt ingest failed");
      } finally {
        setBusy(false);
      }
    },
    [refreshPurchases],
  );

  const submitFile = useCallback(
    async (file: File) => {
      setBusy(true);
      try {
        await uploadReceipt(file);
        await refreshPurchases();
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Upload failed");
      } finally {
        setBusy(false);
      }
    },
    [refreshPurchases],
  );

  const checkPurchase = useCallback(
    async (id: string) => {
      setBusy(true);
      try {
        await runManualCheck(id);
        await refreshPurchases();
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Manual check failed");
      } finally {
        setBusy(false);
      }
    },
    [refreshPurchases],
  );

  const removePurchase = useCallback(
    async (id: string) => {
      setBusy(true);
      try {
        await deletePurchase(id);
        await refreshPurchases();
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Delete failed");
      } finally {
        setBusy(false);
      }
    },
    [refreshPurchases],
  );

  const approveClaimAction = useCallback(
    async (id: string) => {
      setBusy(true);
      try {
        await approveClaim(id);
        await refreshPurchases();
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Claim approval failed");
      } finally {
        setBusy(false);
      }
    },
    [refreshPurchases],
  );

  useEffect(() => {
    const drop = [...events].reverse().find((e) => e.type === "PRICE_DROP_DETECTED");
    if (drop && drop.id !== lastDropIdRef.current) {
      lastDropIdRef.current = drop.id;
      setJustRecovered(true);
      const t = setTimeout(() => setJustRecovered(false), 3500);
      return () => clearTimeout(t);
    }
  }, [events]);

  const agentStatuses = useMemo<AgentStatuses>(() => {
    const last = (type: EventType) => [...events].reverse().find((e) => e.type === type);

    const checkStart  = last("PRICE_CHECK_STARTED");
    const priceUpdate = last("PRICE_UPDATED");
    const drop        = last("PRICE_DROP_DETECTED");
    const payment     = last("PAYMENT_TRIGGERED");
    const policyDone  = last("POLICY_ANALYZED");
    const claimPend   = last("CLAIM_PENDING");
    const claimApproved = last("CLAIM_APPROVED");
    const claimDone   = last("CLAIM_INITIATED");

    // Monitor
    let monitor: AgentStatus = { level: "idle", detail: "Ready to monitor" };
    if (checkStart) {
      const running = !priceUpdate || checkStart.timestamp > priceUpdate.timestamp;
      if (running) {
        monitor = { level: "running", detail: `Checking ${String(checkStart.payload.product ?? "product")}…` };
      } else if (priceUpdate) {
        const obs = priceUpdate.payload.observation as PriceObservation | undefined;
        if (obs) {
          monitor = { level: "done", detail: `${money(obs.price)} via ${obs.source ?? "web"}` };
        } else {
          monitor = { level: "error", detail: String(priceUpdate.payload.error ?? "Check failed") };
        }
      }
    }

    // Recovery
    let recovery: AgentStatus = { level: "idle", detail: "Watching for price drops" };
    if (drop) {
      const amt = typeof drop.payload.recovery_amount === "number" ? money(drop.payload.recovery_amount) : "";
      recovery = payment
        ? { level: "done",    detail: `${amt} dossier filed` }
        : { level: "running", detail: `Processing ${amt} drop…` };
    }

    // Policy
    let policy: AgentStatus = { level: "idle", detail: "Awaiting recovery signal" };
    if (policyDone) {
      const p = policyDone.payload.policy as { retailer?: string; eligible?: boolean; window_days?: number } | undefined;
      policy = p?.eligible
        ? { level: "done", detail: `${p.retailer} eligible · ${p.window_days}d window` }
        : { level: "done", detail: `${p?.retailer ?? "Retailer"} — not eligible` };
    } else if (payment) {
      policy = { level: "running", detail: "Reading retailer T&C…" };
    }

    // Claim
    let claim: AgentStatus = { level: "idle", detail: "Awaiting policy analysis" };
    if (claimDone) {
      claim = { level: "done",    detail: "Claim email dispatched" };
    } else if (claimApproved) {
      claim = { level: "running", detail: "Sending email to retailer…" };
    } else if (claimPend) {
      claim = { level: "waiting", detail: "Awaiting your approval" };
    } else if (policyDone) {
      claim = { level: "waiting", detail: "Draft ready — approve to send" };
    }

    return { monitor, recovery, policy, claim };
  }, [events]);

  const activeStage = useMemo(() => {
    return events.reduce((max, event) => {
      const index = eventOrder.indexOf(event.type);
      return index > max ? index : max;
    }, 0);
  }, [events]);

  const lastNimbleObservation = useMemo(() => {
    for (let i = events.length - 1; i >= 0; i--) {
      const e = events[i];
      if (e.type === "PRICE_UPDATED") {
        const obs = e.payload.observation as PriceObservation | undefined;
        if (obs?.source === "nimble") return obs;
      }
    }
    return null;
  }, [events]);

  const spanCount = useMemo(() => {
    const checks = events.filter((e) => e.type === "PRICE_CHECK_STARTED").length;
    const drops = events.filter((e) => e.type === "PRICE_DROP_DETECTED").length;
    const ingests = events.filter((e) => e.type === "PURCHASE_INGESTED").length;
    return ingests + checks * 2 + drops * 3;
  }, [events]);

  return {
    activeStage,
    agentStatuses,
    approveClaim: approveClaimAction,
    justRecovered,
    busy,
    checkPurchase,
    connected,
    error,
    events,
    eventCount: events.length,
    lapdog,
    lastNimbleObservation,
    purchases,
    refreshPurchases,
    removePurchase,
    spanCount,
    submitFile,
    submitReceipt,
  };
}

function dedupeEvents(events: AgentEvent[]) {
  const seen = new Set<string>();
  return events.filter((event) => {
    if (seen.has(event.id)) return false;
    seen.add(event.id);
    return true;
  });
}

function sortPurchases(purchases: PurchaseSnapshot[]) {
  return purchases
    .slice()
    .sort(
      (a, b) =>
        new Date(b.purchase.created_at).getTime() - new Date(a.purchase.created_at).getTime(),
    );
}
