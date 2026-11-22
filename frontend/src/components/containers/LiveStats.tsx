import type { ReactNode } from "react";
import { formatBytes } from "./format";
import { Sparkline } from "./Sparkline";
import { useLiveStats } from "./useLiveStats";
import { cpuCores, formatMemoryUsage, formatPercent } from "@/lib/statsFormat";

const NA = <span title="Not available on this Engine" className="text-muted-foreground">—</span>;

function Tile({ label, value, sub, children }: { label: ReactNode; value: ReactNode; sub?: ReactNode; children?: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col rounded-[10px] border border-border bg-card p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-lg font-semibold tabular-nums">{value}</div>
      <div className="mt-0.5 min-h-4 text-[11px] text-muted-foreground">{sub}</div>
      <div className="mt-auto pt-2">{children}</div>
    </div>
  );
}

function Legend({ color, text }: { color: string; text: string }) {
  return (
    <span className="inline-flex items-center gap-1 text-[11px]">
      <span className={`h-0.5 w-2.5 rounded ${color}`} />
      {text}
    </span>
  );
}

function Pair({ a, b }: { a: ReactNode; b: ReactNode }) {
  return <div className="flex gap-3">{a}{b}</div>;
}

const rate = (v: number | null | undefined) => (v == null ? "…" : `${formatBytes(Math.round(v))}/s`);

/** Four live tiles for the Overview: CPU, memory (with limit bar), network (with rates), disk and processes. */
export function LiveStats({ connectionId, containerId, running }: { connectionId: string; containerId: string; running: boolean }) {
  const s = useLiveStats(connectionId, containerId, running);
  if (!running) {
    return (
      <p className="rounded-[10px] border border-dashed border-border px-3 py-2.5 text-xs text-muted-foreground">
        Live CPU, memory and network appear here while the container is running.
      </p>
    );
  }
  const d = s.data;
  const wait = s.isLoading ? "…" : NA;
  const cpuVals = s.history.map((p) => p.cpu);
  const memVals = s.history.map((p) => p.mem);
  const cpuMax = Math.max(100, ...cpuVals.map((v) => v ?? 0));
  const hasLimit = !!d?.memoryLimit && d.memoryLimit > 0;
  const memMax = hasLimit ? (d?.memoryLimit as number) : Math.max(1, ...memVals.map((v) => v ?? 0));
  const mem = d?.memoryBytes != null ? formatMemoryUsage(d.memoryBytes, d.memoryLimit) : undefined;
  const rx = s.history.map((p) => p.rxRate);
  const tx = s.history.map((p) => p.txRate);
  const netMax = Math.max(1, ...rx.map((v) => v ?? 0), ...tx.map((v) => v ?? 0));
  const last = s.history[s.history.length - 1];

  return (
    <div className="space-y-2">
      {s.error && <p className="text-sm text-destructive">Could not read stats: {String(s.error)}</p>}
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Tile
          label="CPU"
          value={d?.cpuPercent != null ? `${d.cpuPercent.toFixed(1)}%` : wait}
          sub={d?.cpuPercent != null ? cpuCores(d.cpuPercent) : undefined}
        >
          <Sparkline values={cpuVals} max={cpuMax} label="CPU usage history" />
        </Tile>
        <Tile
          label="Memory"
          value={mem ? mem.text : wait}
          sub={mem?.percent != null ? `${formatPercent(mem.percent)} of limit` : d ? "No memory limit" : undefined}
        >
          <Sparkline values={memVals} max={memMax} label="Memory usage history" />
          {hasLimit && mem?.percent != null && (
            <div className="mt-1.5 h-1 overflow-hidden rounded-full bg-muted" aria-hidden>
              <div className="h-full rounded-full bg-primary" style={{ width: `${Math.min(100, mem.percent)}%` }} />
            </div>
          )}
        </Tile>
        <Tile
          label={<span className="flex items-center gap-2">Network <Legend color="bg-sky-400" text="in" /> <Legend color="bg-emerald-400" text="out" /></span>}
          value={<span className="text-base">↓ {rate(last?.rxRate)} <span className="text-muted-foreground">•</span> ↑ {rate(last?.txRate)}</span>}
          sub={d?.netRxBytes != null ? `total ↓ ${formatBytes(d.netRxBytes)} · ↑ ${formatBytes(d.netTxBytes ?? 0)}` : undefined}
        >
          <div className="relative">
            <Sparkline values={rx} max={netMax} label="Network receive rate history" className="text-sky-400" />
            <div className="absolute inset-0">
              <Sparkline values={tx} max={netMax} label="Network send rate history" className="text-emerald-400" />
            </div>
          </div>
        </Tile>
        <Tile
          label="Disk I/O"
          value={
            d?.blockReadBytes != null ? (
              <Pair a={<span>R {formatBytes(d.blockReadBytes)}</span>} b={<span>W {formatBytes(d.blockWriteBytes ?? 0)}</span>} />
            ) : (
              wait
            )
          }
          sub={d?.pids != null ? `${d.pids} process${d.pids === 1 ? "" : "es"}` : undefined}
        />
      </div>
      <p className="text-[11px] text-muted-foreground">Sampled every 2s while this page is visible · history covers this session only · totals are since the container started.</p>
    </div>
  );
}
