import { useMemo } from "react";
import { Link } from "react-router-dom";
import type { Container } from "@bindings/model";
import { Skeleton } from "@/components/ui/skeleton";
import { Card } from "@/components/system/parts";
import { useContainerStats } from "@/hooks/useContainers";
import { cpuCores, formatMemoryUsage, formatPercent, topConsumers, type Consumer } from "@/lib/statsFormat";
import { formatBytes } from "@/components/containers/format";

function List({ title, rows, render }: { title: string; rows: Consumer[]; render: (c: Consumer) => React.ReactNode }) {
  return (
    <div className="min-w-0">
      <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{title}</h3>
      {rows.length === 0 ? (
        <p className="py-2 text-[13px] text-muted-foreground">No sample for any container yet.</p>
      ) : (
        <ul className="divide-y divide-border/60">
          {rows.map((r) => (
            <li key={r.container.id} className="flex h-[34px] items-center gap-3 text-[13px]">
              <Link to={`/containers/${r.container.id}`} className="min-w-0 flex-1 truncate font-medium hover:text-primary hover:underline" title={r.container.name}>
                {r.container.name}
              </Link>
              <span className="shrink-0 tabular-nums text-muted-foreground">{render(r)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** Top 5 running containers by CPU and by memory, from the live stats watch (no sample = not listed). */
export function TopConsumersCard({ connectionId, containers }: { connectionId: string; containers: readonly Container[] | null | undefined }) {
  const running = useMemo(() => (containers ?? []).filter((c) => c.connectionId === connectionId && c.state === "running"), [containers, connectionId]);
  const ids = useMemo(() => running.map((c) => c.id), [running]);
  const { stats, error } = useContainerStats(connectionId, ids, true);
  const cpu = useMemo(() => topConsumers(running, stats, "cpu"), [running, stats]);
  const mem = useMemo(() => topConsumers(running, stats, "memory"), [running, stats]);

  return (
    <Card title="Top consumers" subtitle="Running containers, live sample">
      {!containers ? (
        <Skeleton className="h-32 w-full" />
      ) : running.length === 0 ? (
        <p className="text-[13px] text-muted-foreground">No running containers.</p>
      ) : error ? (
        <p className="text-[13px] text-destructive">Could not read stats: {error.message}</p>
      ) : stats.size === 0 ? (
        <p className="text-[13px] text-muted-foreground">Collecting…</p>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          <List title="CPU" rows={cpu} render={(r) => `${formatPercent(r.value)} · ${cpuCores(r.value)}`} />
          <List
            title="Memory"
            rows={mem}
            render={(r) => {
              const m = formatMemoryUsage(r.value, stats.get(r.container.id)?.memoryLimit);
              return m.percent != null ? `${m.text} · ${formatPercent(m.percent)}` : formatBytes(r.value);
            }}
          />
        </div>
      )}
    </Card>
  );
}
