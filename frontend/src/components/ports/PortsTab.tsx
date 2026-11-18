import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Info, Network, RefreshCw, Search } from "lucide-react";
import { filterEntries } from "@/lib/portCheck";
import type { Entry } from "@bindings/ports";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/EmptyState";
import { CopyValue } from "@/components/shared/CopyValue";
import { Card } from "@/components/system/parts";
import { usePortListing } from "@/hooks/usePortListing";
import { cn } from "@/lib/utils";

/** System › Ports: who holds which host port, for containers and (when the Engine is local) programs on this computer. */
export function PortsTab({ connectionId }: { connectionId: string }) {
  const [params] = useSearchParams();
  const [query, setQuery] = useState(params.get("q") ?? "");
  const q = usePortListing(connectionId);
  const entries = q.data?.entries ?? [];
  const shown = useMemo(() => filterEntries(entries, query), [entries, query]);

  return (
    <Card
      title="Host ports"
      subtitle={q.data ? `${entries.length} ports${query && shown.length !== entries.length ? `, ${shown.length} match` : ""}` : "Published container ports and listening programs"}
      actions={
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Port, program or container" aria-label="Search ports" className="h-8 w-[240px] pl-8" />
          </div>
          <Button size="sm" variant="outline" onClick={() => void q.refetch()} disabled={q.isFetching}>
            <RefreshCw className={cn("size-3.5", q.isFetching && "animate-spin")} /> Refresh
          </Button>
        </div>
      }
    >
      {q.isLoading ? (
        <Skeleton className="h-60 w-full" />
      ) : q.error ? (
        <EmptyState icon={<Network />} title="Could not read the ports">
          <p className="text-destructive">{String(q.error)}</p>
          <Button className="mt-3" onClick={() => void q.refetch()}>Retry</Button>
        </EmptyState>
      ) : (
        <div className="space-y-3">
          {(q.data?.notes ?? []).map((n) => (
            <p key={n} className="flex items-start gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-[13px] text-muted-foreground">
              <Info className="mt-0.5 size-4 shrink-0" /> {n}
            </p>
          ))}
          {shown.length === 0 ? (
            <EmptyState icon={<Network />} title={query ? "Nothing matches" : "No ports in use"}>
              <p>{query ? "No published port or listener matches the search." : "No container publishes a port and no listener was found."}</p>
            </EmptyState>
          ) : (
            <PortTable entries={shown} />
          )}
        </div>
      )}
    </Card>
  );
}

function PortTable({ entries }: { entries: Entry[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[720px] border-collapse text-[13px]">
        <thead>
          <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
            <th className="h-9 w-24 px-2 font-medium">Port</th>
            <th className="w-36 px-2 font-medium">Address</th>
            <th className="px-2 font-medium">Held by</th>
            <th className="w-44 px-2 font-medium">Where</th>
            <th className="px-2 font-medium">Path</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((e, i) => (
            <tr key={`${e.hostPort}-${e.protocol}-${e.hostIp}-${e.holder.pid}-${e.holder.containerId}-${i}`} className="h-[38px] border-b border-border/60">
              <td className="px-2 font-mono tabular-nums">
                {e.hostPort}
                <span className="text-muted-foreground">/{e.protocol}</span>
              </td>
              <td className="px-2 font-mono text-xs text-muted-foreground">{e.hostIp || "all"}</td>
              <td className="max-w-[300px] px-2">
                <Holder entry={e} />
              </td>
              <td className="px-2 text-muted-foreground">{e.holder.location || "—"}</td>
              <td className="max-w-[320px] px-2">
                {e.holder.executablePath ? <CopyValue value={e.holder.executablePath} what="Path" /> : <span className="text-muted-foreground">—</span>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Holder({ entry }: { entry: Entry }) {
  const h = entry.holder;
  if (h.kind === "container") {
    return (
      <span className="flex min-w-0 items-baseline gap-1.5">
        <Link to={`/containers/${h.containerId}`} className="truncate font-medium hover:underline" title={h.containerName}>
          {h.containerName}
        </Link>
        {!h.running && <span className="shrink-0 text-xs text-muted-foreground">stopped</span>}
        {h.composeProject && <span className="shrink-0 text-xs text-muted-foreground">{h.composeProject}</span>}
      </span>
    );
  }
  return (
    <span className="flex min-w-0 items-baseline gap-1.5">
      <span className={cn("truncate font-medium", !h.processName && "font-normal text-muted-foreground")} title={h.processName}>
        {h.processName || "Not identified"}
      </span>
      {h.pid > 0 && <span className="shrink-0 text-xs tabular-nums text-muted-foreground">PID {h.pid}</span>}
      {h.kind === "system" && <span className="shrink-0 text-xs text-muted-foreground">system</span>}
    </span>
  );
}
