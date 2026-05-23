"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AgentEvent,
  fetchEvents,
  fetchLapdogInfo,
  fetchPurchases,
  ingestReceipt,
  LapdogInfo,
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
  const [lapdog, setLapdog] = useState<LapdogInfo>({ running: false });

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
          event.type === "PRICE_UPDATED" ||
          event.type === "PRICE_DROP_DETECTED" ||
          event.type === "PAYMENT_TRIGGERED"
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
    return events.reduce((max, event) => {
      const index = eventOrder.indexOf(event.type);
      return index > max ? index : max;
    }, 0);
  }, [events]);

  return {
    activeStage,
    busy,
    checkPurchase,
    connected,
    error,
    events,
    lapdog,
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

function sortPurchases(purchases: PurchaseSnapshot[]) {
  return purchases
    .slice()
    .sort(
      (a, b) =>
        new Date(b.purchase.created_at).getTime() - new Date(a.purchase.created_at).getTime(),
    );
}
