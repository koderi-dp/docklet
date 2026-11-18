import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ContainerService } from "@bindings/services";
import type { ContainerStatDetail } from "@bindings/model";
import { EmptyState } from "@/components/shared/EmptyState";
import { formatBytes } from "./format";
import { cpuCores, formatMemoryUsage, formatPercent } from "@/lib/statsFormat";
import { usePageVisible } from "@/hooks/queryUtils";

const POLL_MS = 2000;
const HISTORY = 90; // ~3 minutes at 2s

interface Point {
  cpu: number | null;
  mem: number | null;
}

/** Inline SVG polyline; null values break the line (never drawn as zero). */
export function Sparkline({ values, max, label }: { values: Array<number | null>; max: number; label: string }) {
  const W = 240;
  const H = 48;
  const n = Math.max(values.length, 2);
  const segs: string[] = [];
  let cur: string[] = [];
  values.forEach((v, i) => {
    if (v == null) {
      if (cur.length) segs.push(cur.join(" "));
      cur = [];
      return;
    }
    const x = (i / (n - 1)) * W;
    const y = H - 2 - (Math.min(v, max) / (max || 1)) * (H - 4);
    cur.push(`${x.toFixed(1)},${y.toFixed(1)}`);
  });
  if (cur.length) segs.push(cur.join(" "));
  return (
    <svg viewBox={`0 0 ${W} ${H}`} className="h-12 w-full text-primary" role="img" aria-label={label} preserveAspectRatio="none">
      <line x1="0" y1={H - 1} x2={W} y2={H - 1} className="stroke-border" strokeWidth="1" />
      {segs.map((pts, i) =>
        pts.includes(" ") ? (
          <polyline key={i} points={pts} fill="none" stroke="currentColor" strokeWidth="1.5" vectorEffect="non-scaling-stroke" />
        ) : (
          <circle key={i} cx={pts.split(",")[0]} cy={pts.split(",")[1]} r="1.5" fill="currentColor" />
        ),
      )}
    </svg>
  );
}

function memoryText(used: number, limit?: number | null): string {
  const m = formatMemoryUsage(used, limit);
  return m.percent != null ? `${m.text} · ${formatPercent(m.percent)}` : m.text;
}

const NA = <span title="Not available on this Engine">Not available</span>;

function Tile({ label, value, children }: { label: string; value: React.ReactNode; children?: React.ReactNode }) {
  return (
    <div className="rounded-[10px] border border-border bg-card p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-lg font-semibold tabular-nums">{value}</div>
      {children}
    </div>
  );
}

/** Live stats, polled every 2s only while mounted (tab selected) and the window is visible. */
export function StatsPanel({ connectionId, containerId, running }: { connectionId: string; containerId: string; running: boolean }) {
  const visible = usePageVisible();
  const q = useQuery({
    queryKey: ["container-stat-detail", connectionId, containerId],
    queryFn: () => ContainerService.StatsDetail(connectionId, containerId),
    enabled: running && visible,
    refetchInterval: POLL_MS,
    refetchIntervalInBackground: false,
    retry: false,
  });
  const [history, setHistory] = useState<Point[]>([]);
  const lastAt = useRef(0);
  useEffect(() => {
    if (!q.data || q.dataUpdatedAt === lastAt.current) return;
    lastAt.current = q.dataUpdatedAt;
    const d = q.data;
    setHistory((h) => [...h, { cpu: d.cpuPercent ?? null, mem: d.memoryBytes ?? null }].slice(-HISTORY));
  }, [q.data, q.dataUpdatedAt]);

  if (!running)
    return (
      <EmptyState title="Container is not running">
        <p>Live stats are available only while the container is running.</p>
      </EmptyState>
    );

  const d: ContainerStatDetail | undefined = q.data;
  const cpuVals = history.map((p) => p.cpu);
  const memVals = history.map((p) => p.mem);
  const cpuMax = Math.max(100, ...cpuVals.map((v) => v ?? 0));
  const memMax = d?.memoryLimit && d.memoryLimit > 0 ? d.memoryLimit : Math.max(1, ...memVals.map((v) => v ?? 0));

  return (
    <div className="space-y-3">
      {q.error && <p className="text-sm text-destructive">Could not read stats: {String(q.error)}</p>}
      <div className="grid gap-3 md:grid-cols-2">
        <Tile label="CPU" value={d?.cpuPercent != null ? `${d.cpuPercent.toFixed(1)}% · ${cpuCores(d.cpuPercent)}` : q.isLoading ? "…" : NA}>
          <Sparkline values={cpuVals} max={cpuMax} label="CPU usage history" />
          <div className="text-[11px] text-muted-foreground">Scale 0–{cpuMax.toFixed(0)}% · this session only</div>
        </Tile>
        <Tile
          label="Memory"
          value={
            d?.memoryBytes != null
              ? memoryText(d.memoryBytes, d.memoryLimit)
              : q.isLoading
                ? "…"
                : NA
          }
        >
          <Sparkline values={memVals} max={memMax} label="Memory usage history" />
          <div className="text-[11px] text-muted-foreground">
            Scale 0–{formatBytes(memMax)}
            {d?.memoryLimit ? "" : " (no memory limit reported)"} · this session only
          </div>
        </Tile>
      </div>
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Tile label="Network RX" value={d?.netRxBytes != null ? formatBytes(d.netRxBytes) : q.isLoading ? "…" : NA} />
        <Tile label="Network TX" value={d?.netTxBytes != null ? formatBytes(d.netTxBytes) : q.isLoading ? "…" : NA} />
        <Tile label="Block read" value={d?.blockReadBytes != null ? formatBytes(d.blockReadBytes) : q.isLoading ? "…" : NA} />
        <Tile label="Block write" value={d?.blockWriteBytes != null ? formatBytes(d.blockWriteBytes) : q.isLoading ? "…" : NA} />
        <Tile label="Processes (PIDs)" value={d?.pids != null ? String(d.pids) : q.isLoading ? "…" : NA} />
      </div>
      <p className="text-xs text-muted-foreground">
        Network and block I/O are cumulative totals since the container started. Sampled every 2s while this tab is visible.
      </p>
    </div>
  );
}
