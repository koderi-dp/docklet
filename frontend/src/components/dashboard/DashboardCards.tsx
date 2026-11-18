import { Link } from "react-router-dom";
import type { Container, DiskUsage, EngineEvent, EngineInfo } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ContainerStatusBadge } from "@/components/containers/ContainerStatusBadge";
import { formatBytes } from "@/components/containers/format";
import { Bytes, Card, InfoGrid, NotAvailable, formatEventTime } from "@/components/system/parts";
import { attrSummary } from "@/components/system/EventsTab";
import { engineName } from "@/lib/engineKind";

type Q<T> = { data?: T | null; isLoading: boolean; error: unknown };

function Failed({ what, error }: { what: string; error: unknown }) {
  return <p className="text-[13px] text-destructive">Could not load {what}: {String(error)}</p>;
}

export function EngineCard({ info }: { info: Q<EngineInfo> }) {
  return (
    <Card title="Engine" subtitle={`${engineName(info.data?.kind)} engine of this connection`} actions={<span className="rounded-md border border-success/30 bg-success/10 px-2 py-0.5 text-xs font-medium text-success">Connected</span>}>
      {info.isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : info.error || !info.data ? (
        <Failed what="Engine information" error={info.error} />
      ) : (
        <InfoGrid
          items={[
            { label: `${engineName(info.data.kind)} version`, value: info.data.serverVersion },
            { label: "API version", value: info.data.apiVersion },
            { label: "Operating system", value: info.data.operatingSystem },
            { label: "Architecture", value: info.data.architecture },
            { label: "CPUs", value: info.data.ncpu ? String(info.data.ncpu) : "" },
            { label: "Total memory", value: info.data.memTotal ? formatBytes(info.data.memTotal) : "" },
          ]}
        />
      )}
    </Card>
  );
}

export function ResourceUsageCard({ info, disk }: { info: Q<EngineInfo>; disk: Q<DiskUsage> }) {
  return (
    <Card title="Resource usage" subtitle={`${engineName(info.data?.kind)} disk usage (system df) and host capacity. Not host disk or live CPU utilisation.`}>
      <div className="space-y-4">
        <div>
          <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Host capacity</h3>
          <dl className="grid grid-cols-2 gap-3 text-[13px]">
            <div>
              <dt className="text-muted-foreground">CPUs</dt>
              <dd className="text-lg font-semibold tabular-nums">{info.data?.ncpu ? info.data.ncpu : <NotAvailable reason="The Engine did not report a CPU count" />}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Total memory</dt>
              <dd className="text-lg font-semibold">{info.data?.memTotal ? formatBytes(info.data.memTotal) : <NotAvailable reason="The Engine did not report total memory" />}</dd>
            </div>
          </dl>
        </div>
        <div>
          <h3 className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{engineName(info.data?.kind)} disk usage</h3>
          {disk.isLoading ? (
            <Skeleton className="h-24 w-full" />
          ) : disk.error || !disk.data ? (
            <Failed what={`${engineName(info.data?.kind)} disk usage`} error={disk.error} />
          ) : (
            <table className="w-full text-[13px]">
              <thead>
                <tr className="text-left text-xs text-muted-foreground">
                  <th className="h-8 font-medium">Category</th>
                  <th className="text-right font-medium">Size</th>
                  <th className="text-right font-medium">Reclaimable</th>
                </tr>
              </thead>
              <tbody>
                {(disk.data.categories ?? []).map((c) => (
                  <tr key={c.key} className="border-t border-border/60">
                    <td className="h-8">{c.label}</td>
                    <td className="text-right"><Bytes value={c.size} /></td>
                    <td className="text-right"><Bytes value={c.reclaimable} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </Card>
  );
}

export function RecentContainersCard({ containers, onViewAll }: { containers: Q<Container[]>; onViewAll: () => void }) {
  const rows = [...(containers.data ?? [])].sort((a, b) => b.created - a.created).slice(0, 5);
  return (
    <Card title="Recent containers" subtitle="Most recently created" actions={<Button size="sm" variant="ghost" onClick={onViewAll}>View all</Button>}>
      {containers.isLoading ? (
        <Skeleton className="h-32 w-full" />
      ) : containers.error ? (
        <Failed what="containers" error={containers.error} />
      ) : rows.length === 0 ? (
        <p className="text-[13px] text-muted-foreground">No containers on this Engine.</p>
      ) : (
        <ul className="divide-y divide-border/60">
          {rows.map((c) => (
            <li key={c.id} className="flex h-[42px] items-center gap-3 text-[13px]">
              <Link to={`/containers/${c.id}`} className="min-w-0 flex-1 truncate font-medium hover:text-primary hover:underline" title={c.name}>
                {c.name}
              </Link>
              <span className="hidden max-w-[160px] truncate text-muted-foreground sm:block" title={c.image}>{c.image}</span>
              <ContainerStatusBadge container={c} />
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

export function RecentEventsCard({ events }: { events: Q<EngineEvent[]> }) {
  const rows = (events.data ?? []).slice(0, 8);
  return (
    <Card title="Recent events" subtitle="Last hour, newest first">
      {events.isLoading ? (
        <Skeleton className="h-32 w-full" />
      ) : events.error ? (
        <Failed what="events" error={events.error} />
      ) : rows.length === 0 ? (
        <p className="text-[13px] text-muted-foreground">No events in the last hour.</p>
      ) : (
        <ul className="divide-y divide-border/60">
          {rows.map((e, i) => (
            <li key={`${e.time}-${e.actorId}-${i}`} className="flex h-[38px] items-center gap-3 text-[13px]">
              <span className="w-[64px] shrink-0 tabular-nums text-muted-foreground">{formatEventTime(e.time)}</span>
              <span className="w-[70px] shrink-0 text-muted-foreground">{e.type}</span>
              <span className="min-w-0 flex-1 truncate" title={`${e.action} ${attrSummary(e)}`}>
                <span className="font-medium">{e.actorName}</span> <span className="text-muted-foreground">{e.action}</span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
