"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AgentEvent,
  fetchPurchases,
  ingestReceipt,
  PurchaseSnapshot,
  runManualCheck,
  subscribeToEvents,
} from "./reclaimo";

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

  const refreshPurchases = useCallback(async () => {
    try {
      setPurchases(await fetchPurchases());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load purchases");
    }
  }, []);

  useEffect(() => {
    refreshPurchases();
    const interval = window.setInterval(refreshPurchases, 2500);
    return () => window.clearInterval(interval);
  }, [refreshPurchases]);

  useEffect(() => {
    const close = subscribeToEvents((event) => {
      setConnected(true);
      setEvents((current) => dedupeEvents([...current, event]).slice(-160));
      if (
        event.type === "PURCHASE_INGESTED" ||
        event.type === "PRICE_UPDATED" ||
        event.type === "PRICE_DROP_DETECTED" ||
        event.type === "PAYMENT_TRIGGERED"
      ) {
        refreshPurchases();
      }
    });
    return close;
  }, [refreshPurchases]);

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

  const activeStage = useMemo(() => {
    const latest = events.at(-1);
    if (!latest) return 0;
    return Math.max(0, eventOrder.indexOf(latest.type));
  }, [events]);

  return {
    activeStage,
    busy,
    checkPurchase,
    connected,
    error,
    events,
    purchases,
    refreshPurchases,
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

