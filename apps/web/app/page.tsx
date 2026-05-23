"use client";

import {
  Activity,
  Check,
  CircleDollarSign,
  ClipboardCheck,
  Database,
  FileText,
  Globe2,
  Mail,
  Radar,
  ReceiptText,
  RefreshCw,
  Send,
  Terminal,
  Trash2,
  Upload,
  Zap,
} from "lucide-react";
import { useMemo, useRef, useState } from "react";
import { ClaimPacket, compactTime, eventSummary, money, PolicyAnalysis, PriceObservation, PurchaseSnapshot } from "../lib/reclaimo";
import { AgentStatuses, AgentStatusLevel, useReclaimo } from "../lib/use-reclaimo";

const sampleReceipt =
  "Thanks for your order from Amazon\nMacBook Pro 14 M4\nPrice: $2199\nOrder ID: DEMO-9001";

const stages = [
  { label: "Ingest", type: "PURCHASE_INGESTED", icon: ReceiptText },
  { label: "Extract", type: "PRODUCT_EXTRACTED", icon: ClipboardCheck },
  { label: "Monitor", type: "PRICE_CHECK_STARTED", icon: Radar },
  { label: "Update", type: "PRICE_UPDATED", icon: Globe2 },
  { label: "Detect", type: "PRICE_DROP_DETECTED", icon: Activity },
  { label: "Dossier", type: "RECOVERY_REPORT_GENERATED", icon: Send },
  { label: "Publish", type: "RECOVERY_PUBLISHED", icon: Check },
  { label: "Payment", type: "PAYMENT_TRIGGERED", icon: CircleDollarSign },
  { label: "Policy", type: "POLICY_ANALYZED", icon: FileText },
  { label: "Claim", type: "CLAIM_PENDING", icon: Mail },
];

export default function Home() {
  const {
    activeStage,
    agentStatuses,
    busy,
    checkPurchase,
    connected,
    error,
    eventCount,
    events,
    justRecovered,
    lapdog,
    lastNimbleObservation,
    purchases,
    removePurchase,
    spanCount,
    submitFile,
  } = useReclaimo();

  const recoveredTotal = useMemo(
    () =>
      purchases.reduce((sum, item) => {
        const baseline = item.purchase.baseline_price;
        const observed = item.last_observed?.price ?? baseline;
        return ["recovered", "pending_claim", "claim_submitted"].includes(item.status)
          ? sum + Math.max(0, baseline - observed)
          : sum;
      }, 0),
    [purchases],
  );

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Email-first autonomous recovery</p>
          <h1>Reclaimo</h1>
        </div>
        <div className="status-strip" aria-label="Backend connection status">
          <span className={connected ? "pulse online" : "pulse"} />
          <span>{connected ? "Backend online" : "Waiting for backend"}</span>
        </div>
      </header>

      <section className="mode-strip" aria-label="Agent mode">
        <div>
          <Zap size={16} />
          <span>Autonomous Agent Mode</span>
        </div>
        <p>Receipt ingestion starts monitoring automatically. Live Nimble observations appear as source badges.</p>
      </section>

      {error ? <div className="error-band">{error}</div> : null}

      <section className="workbench">
        <aside className="panel intake-panel">
          <div className="panel-heading">
            <ReceiptText size={18} />
            <h2>Receipt Intake</h2>
          </div>
          <ReceiptDropZone busy={busy} onFile={submitFile} />

          <div className="metric-grid">
            <div className="metric">
              <span>Purchases</span>
              <strong>{purchases.length}</strong>
            </div>
            <div className={`metric${recoveredTotal > 0 ? " hero" : ""}${justRecovered ? " celebrating" : ""}`}>
              <span>Recovered</span>
              <strong>{money(recoveredTotal)}</strong>
            </div>
          </div>

          <ToolFeed
            lastNimbleObservation={lastNimbleObservation}
            eventCount={eventCount}
            spanCount={spanCount}
            lapdogRunning={lapdog.running}
          />
        </aside>

        <section className="agent-stage">
          <div className="stage-header">
            <div>
              <p className="eyebrow">Living Agent System</p>
              <h2>Autonomous recovery pipeline</h2>
            </div>
            <span className="event-count">{events.length} events</span>
          </div>

          <AgentStatusPanel statuses={agentStatuses} />

          <div className="flow-grid">
            {stages.map((stage, index) => {
              const Icon = stage.icon;
              const active = index <= activeStage;
              return (
                <div className={active ? "flow-node active" : "flow-node"} key={stage.type}>
                  <div className="node-icon">
                    <Icon size={18} />
                  </div>
                  <span title={stage.label}>{stage.label}</span>
                </div>
              );
            })}
          </div>

          <div className="purchase-table">
            {purchases.length === 0 ? (
              <div className="empty-state">Start the Go backend to see seeded purchases.</div>
            ) : (
              purchases.map((item) => (
                <PurchaseRow
                  item={item}
                  key={item.purchase.id}
                  onCheck={checkPurchase}
                  onDelete={removePurchase}
                  busy={busy}
                />
              ))
            )}
          </div>
        </section>

        <aside className="panel recovery-panel">
          <div className="panel-heading">
            <CircleDollarSign size={18} />
            <h2>Recovery Dossiers</h2>
          </div>
          <div className="recovery-list">
            {purchases
              .filter((item) => ["recovered", "pending_claim", "claim_submitted"].includes(item.status))
              .map((item) => (
                <RecoveryCard
                  key={item.purchase.id}
                  item={item}
                />
              ))}
            {purchases.every((item) => !["recovered", "pending_claim", "claim_submitted"].includes(item.status)) ? (
              <div className="empty-state compact">No recovery dossiers yet.</div>
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
                <div>
                  <code>{event.type}</code>
                  <p>{eventSummary(event)}</p>
                </div>
                <span>{event.id}</span>
              </div>
            ))}
          {events.length === 0 ? <div className="empty-state compact">No events received.</div> : null}
        </div>
      </section>
    </main>
  );
}

function ToolFeed({
  lastNimbleObservation,
  eventCount,
  spanCount,
  lapdogRunning,
}: {
  lastNimbleObservation: PriceObservation | null;
  eventCount: number;
  spanCount: number;
  lapdogRunning: boolean;
}) {
  return (
    <div className="tool-feed">
      <div className="tool-row nimble">
        <Globe2 size={14} />
        <strong>Nimble</strong>
        <span>
          {lastNimbleObservation
            ? `${money(lastNimbleObservation.price)} observed · ${compactTime(lastNimbleObservation.timestamp)}`
            : "open web monitor ready"}
        </span>
      </div>
      <div className="tool-row clickhouse">
        <Database size={14} />
        <strong>ClickHouse</strong>
        <span>{eventCount > 0 ? `${eventCount} events written · reclaimo.events` : "event sink ready"}</span>
      </div>
      <div className="tool-row lapdog">
        <Terminal size={14} />
        <strong>Lapdog</strong>
        <span>
          {lapdogRunning
            ? `~${spanCount} spans traced · 127.0.0.1:8126`
            : "agent not detected · run lapdog start"}
        </span>
      </div>
    </div>
  );
}

function ReceiptDropZone({ busy, onFile }: { busy: boolean; onFile: (f: File) => void }) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const [fileName, setFileName] = useState<string | null>(null);

  function handle(file: File | null | undefined) {
    if (!file) return;
    setFileName(file.name);
    onFile(file);
  }

  return (
    <div
      className={`receipt-dropzone${dragging ? " dragging" : ""}${fileName ? " has-file" : ""}`}
      onClick={() => inputRef.current?.click()}
      onDragOver={(e) => { e.preventDefault(); setDragging(true); }}
      onDragLeave={() => setDragging(false)}
      onDrop={(e) => { e.preventDefault(); setDragging(false); handle(e.dataTransfer.files[0]); }}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => e.key === "Enter" && inputRef.current?.click()}
    >
      <input
        ref={inputRef}
        type="file"
        accept="image/jpeg,image/png,image/webp,application/pdf"
        style={{ display: "none" }}
        onChange={(e) => handle(e.target.files?.[0])}
      />
      {busy ? (
        <>
          <Upload size={22} className="dropzone-icon spinning" />
          <span>Analysing receipt with Claude…</span>
        </>
      ) : fileName ? (
        <>
          <ReceiptText size={22} className="dropzone-icon done" />
          <span>{fileName}</span>
          <small>Click to upload another</small>
        </>
      ) : (
        <>
          <Upload size={22} className="dropzone-icon" />
          <span>Drop receipt image or click to upload</span>
          <small>JPEG · PNG · WEBP · PDF</small>
        </>
      )}
    </div>
  );
}

function PurchaseRow({
  busy,
  item,
  onCheck,
  onDelete,
}: {
  busy: boolean;
  item: PurchaseSnapshot;
  onCheck: (id: string) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
}) {
  const baseline = item.purchase.baseline_price;
  const current = item.last_observed?.price;
  const delta = current ? Math.max(0, baseline - current) : 0;
  const source = sourceLabel(item.last_observed?.source);
  const isMonitoring = item.status === "monitoring";
  const isRecovered = ["recovered", "pending_claim", "claim_submitted"].includes(item.status);

  return (
    <article className={`purchase-row${isRecovered ? " is-recovered" : ""}`}>
      <div className="purchase-main">
        <span className={`status-chip ${item.status}`}>{item.status.replace("_", " ")}</span>
        <h3>{item.purchase.product}</h3>
        <p>
          {money(baseline)} baseline
          {current ? ` · ${money(current)} observed` : ""}
        </p>
        {current ? <span className={`source-badge ${item.last_observed?.source ?? "unknown"}`}>{source}</span> : null}
      </div>
      <div className="purchase-meta">
        <span>{item.check_count} checks</span>
        <strong>{delta > 0 ? money(delta) : "No drop"}</strong>
        {item.deadline ? <span className="deadline-chip">{daysLeft(item.deadline)}d left</span> : null}
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
      <button
        aria-label={`Delete ${item.purchase.product}`}
        className="icon-action danger"
        disabled={busy}
        onClick={() => onDelete(item.purchase.id)}
        type="button"
      >
        <Trash2 size={17} />
      </button>
      {isMonitoring ? <div className="monitoring-bar" /> : null}
    </article>
  );
}

function RecoveryCard({ item }: { item: PurchaseSnapshot }) {
  const recoveryAmount =
    item.purchase.baseline_price - (item.last_observed?.price ?? item.purchase.baseline_price);
  const policy = item.policy_analysis;
  const claim = item.claim_packet;
  const isPending = item.status === "pending_claim";
  const isClaimed = item.status === "claim_submitted";

  return (
    <div className={`recovery-item${isPending ? " pending-claim" : ""}${isClaimed ? " claim-submitted" : ""}`}>
      <div className="recovery-item-header">
        <span>{item.purchase.product}</span>
        <strong>{money(recoveryAmount)}</strong>
      </div>
      <small>{sourceLabel(item.last_observed?.source)} dossier</small>

      {policy ? (
        <div className="policy-row">
          <span className={`policy-badge ${policy.eligible ? "eligible" : "ineligible"}`}>
            {policy.eligible ? `${policy.retailer} eligible` : `${policy.retailer} — not eligible`}
          </span>
          {policy.eligible ? (
            <span className="policy-detail">{policy.window_days}d window · {policy.tat_days}</span>
          ) : null}
        </div>
      ) : null}

      {isPending && claim ? (
        <div className="claim-action">
          <p className="claim-preview">{claim.draft_subject}</p>
          <p className="claim-sending">
            <Mail size={13} />
            Sending claim email…
          </p>
        </div>
      ) : null}

      {isClaimed ? (
        <p className="claim-sent">
          <Check size={13} />
          Email sent {claim?.sent_at ? compactTime(claim.sent_at) : ""}
        </p>
      ) : null}
    </div>
  );
}

const agentDefs = [
  { key: "monitor",  label: "Monitor Agent",  icon: Radar,           desc: "Nimble web scraping" },
  { key: "recovery", label: "Recovery Agent", icon: Activity,        desc: "Dossier + payment rail" },
  { key: "policy",   label: "Policy Agent",   icon: FileText,        desc: "Retailer T&C analysis" },
  { key: "claim",    label: "Claim Agent",    icon: Mail,            desc: "AI-drafted email claim" },
] as const;

const levelColor: Record<AgentStatusLevel, string> = {
  idle:    "idle",
  running: "running",
  waiting: "waiting",
  done:    "done",
  error:   "error",
};

function AgentStatusPanel({ statuses }: { statuses: AgentStatuses }) {
  return (
    <div className="agent-status-panel">
      {agentDefs.map(({ key, label, icon: Icon, desc }) => {
        const status = statuses[key];
        return (
          <div className={`agent-card level-${levelColor[status.level]}`} key={key}>
            <div className="agent-card-icon">
              <Icon size={16} />
              {status.level === "running" && <span className="agent-pulse" />}
            </div>
            <div className="agent-card-body">
              <strong>{label}</strong>
              <span className="agent-desc">{desc}</span>
              <p className="agent-detail">{status.detail}</p>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function daysLeft(deadline: string) {
  const ms = new Date(deadline).getTime() - Date.now();
  return Math.max(0, Math.ceil(ms / 86_400_000));
}

function sourceLabel(source?: string) {
  if (source === "nimble") return "Live Nimble signal";
  if (source === "demo") return "Demo price signal";
  if (source === "test") return "Test signal";
  return "Price signal";
}
