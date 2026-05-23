"use client";

import {
  Activity,
  Check,
  CircleDollarSign,
  ClipboardCheck,
  Globe2,
  Play,
  Radar,
  ReceiptText,
  RefreshCw,
  Send,
  Terminal,
} from "lucide-react";
import { FormEvent, useMemo, useState } from "react";
import { compactTime, money, PurchaseSnapshot } from "../lib/reclaimo";
import { useReclaimo } from "../lib/use-reclaimo";

const sampleReceipt =
  "Thanks for your order from Amazon\nMacBook Pro 14 M4\nPrice: $2199\nOrder ID: DEMO-9001";

const stages = [
  { label: "Ingest", type: "PURCHASE_INGESTED", icon: ReceiptText },
  { label: "Extract", type: "PRODUCT_EXTRACTED", icon: ClipboardCheck },
  { label: "Monitor", type: "PRICE_CHECK_STARTED", icon: Radar },
  { label: "Update", type: "PRICE_UPDATED", icon: Globe2 },
  { label: "Detect", type: "PRICE_DROP_DETECTED", icon: Activity },
  { label: "Report", type: "RECOVERY_REPORT_GENERATED", icon: Send },
  { label: "Publish", type: "RECOVERY_PUBLISHED", icon: Check },
  { label: "Payment", type: "PAYMENT_TRIGGERED", icon: CircleDollarSign },
];

export default function Home() {
  const {
    activeStage,
    busy,
    checkPurchase,
    connected,
    error,
    events,
    purchases,
    submitReceipt,
  } = useReclaimo();
  const [receipt, setReceipt] = useState(sampleReceipt);

  const recoveredTotal = useMemo(
    () =>
      purchases.reduce((sum, item) => {
        const baseline = item.purchase.baseline_price;
        const observed = item.last_observed?.price ?? baseline;
        return item.status === "recovered" ? sum + Math.max(0, baseline - observed) : sum;
      }, 0),
    [purchases],
  );

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await submitReceipt(receipt);
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Email-first autonomous recovery</p>
          <h1>Reclaimo</h1>
        </div>
        <div className="status-strip" aria-label="Backend connection status">
          <span className={connected ? "pulse online" : "pulse"} />
          <span>{connected ? "Agent stream live" : "Waiting for backend"}</span>
        </div>
      </header>

      {error ? <div className="error-band">{error}</div> : null}

      <section className="workbench">
        <aside className="panel intake-panel">
          <div className="panel-heading">
            <ReceiptText size={18} />
            <h2>Receipt Intake</h2>
          </div>
          <form onSubmit={onSubmit}>
            <textarea
              value={receipt}
              onChange={(event) => setReceipt(event.target.value)}
              spellCheck={false}
              aria-label="Paste receipt or order confirmation email"
            />
            <button className="primary-action" disabled={busy || !receipt.trim()} type="submit">
              <Play size={16} />
              <span>{busy ? "Running" : "Start agent"}</span>
            </button>
          </form>

          <div className="metric-grid">
            <Metric label="Purchases" value={String(purchases.length)} />
            <Metric label="Recovered" value={money(recoveredTotal)} />
          </div>
        </aside>

        <section className="agent-stage">
          <div className="stage-header">
            <div>
              <p className="eyebrow">Living Agent System</p>
              <h2>Autonomous recovery pipeline</h2>
            </div>
            <span className="event-count">{events.length} events</span>
          </div>

          <div className="flow-grid">
            {stages.map((stage, index) => {
              const Icon = stage.icon;
              const active = index <= activeStage;
              return (
                <div className={active ? "flow-node active" : "flow-node"} key={stage.type}>
                  <div className="node-icon">
                    <Icon size={18} />
                  </div>
                  <span>{stage.label}</span>
                </div>
              );
            })}
          </div>

          <div className="purchase-table">
            {purchases.length === 0 ? (
              <div className="empty-state">Start the Go backend to see seeded purchases.</div>
            ) : (
              purchases.map((item) => (
                <PurchaseRow item={item} key={item.purchase.id} onCheck={checkPurchase} busy={busy} />
              ))
            )}
          </div>
        </section>

        <aside className="panel recovery-panel">
          <div className="panel-heading">
            <CircleDollarSign size={18} />
            <h2>Recovery Stream</h2>
          </div>
          <div className="recovery-list">
            {purchases
              .filter((item) => item.status === "recovered")
              .map((item) => (
                <div className="recovery-item" key={item.purchase.id}>
                  <span>{item.purchase.product}</span>
                  <strong>
                    {money(
                      item.purchase.baseline_price - (item.last_observed?.price ?? item.purchase.baseline_price),
                    )}
                  </strong>
                </div>
              ))}
            {purchases.every((item) => item.status !== "recovered") ? (
              <div className="empty-state compact">No recovery events yet.</div>
            ) : null}
          </div>
        </aside>
      </section>

      <section className="trace-console">
        <div className="panel-heading">
          <Terminal size={18} />
          <h2>Execution Trace</h2>
        </div>
        <div className="trace-lines">
          {events
            .slice()
            .reverse()
            .slice(0, 40)
            .map((event) => (
              <div className="trace-line" key={event.id}>
                <time>{compactTime(event.timestamp)}</time>
                <code>{event.type}</code>
                <span>{event.id}</span>
              </div>
            ))}
          {events.length === 0 ? <div className="empty-state compact">No events received.</div> : null}
        </div>
      </section>
    </main>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function PurchaseRow({
  busy,
  item,
  onCheck,
}: {
  busy: boolean;
  item: PurchaseSnapshot;
  onCheck: (id: string) => Promise<void>;
}) {
  const baseline = item.purchase.baseline_price;
  const current = item.last_observed?.price;
  const delta = current ? Math.max(0, baseline - current) : 0;

  return (
    <article className="purchase-row">
      <div className="purchase-main">
        <span className={`status-chip ${item.status}`}>{item.status}</span>
        <h3>{item.purchase.product}</h3>
        <p>
          {money(baseline)} baseline
          {current ? ` · ${money(current)} observed` : ""}
        </p>
      </div>
      <div className="purchase-meta">
        <span>{item.check_count} checks</span>
        <strong>{delta > 0 ? money(delta) : "No drop"}</strong>
      </div>
      <button
        aria-label={`Run manual check for ${item.purchase.product}`}
        className="icon-action"
        disabled={busy}
        onClick={() => onCheck(item.purchase.id)}
        type="button"
      >
        <RefreshCw size={17} />
      </button>
    </article>
  );
}

