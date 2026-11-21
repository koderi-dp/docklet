import { useMemo, useState } from "react";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useComposeLogs } from "@/hooks/useCompose";
import { filterComposeLogs, parseComposeLogs } from "@/lib/composeLogs";
import { cn } from "@/lib/utils";
import { ComposeLogList } from "./ComposeLogList";

const TAILS = [200, 500, 1000, 2000];
const selectCls =
  "h-8 rounded-md border border-border bg-background px-2 text-[13px] text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring/60";

/** Merged, timestamp-ordered snapshot of the project's container logs (not a live stream). */
export function LogsTab({ connectionId, project, services, active }: { connectionId: string; project: string; services: string[]; active: boolean }) {
  const [service, setService] = useState("all");
  const [tail, setTail] = useState(500);
  const q = useComposeLogs(connectionId, project, tail, active);
  // Escape codes are parsed once per fetched snapshot; changing the service only filters.
  const parsed = useMemo(() => parseComposeLogs(q.data), [q.data]);
  const lines = useMemo(() => filterComposeLogs(parsed, service), [parsed, service]);

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
          Service
          <select className={selectCls} value={service} onChange={(e) => setService(e.target.value)}>
            <option value="all">All services</option>
            {services.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
          Lines
          <select className={selectCls} value={tail} onChange={(e) => setTail(Number(e.target.value))}>
            {TAILS.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </label>
        <Button size="sm" variant="outline" onClick={() => void q.refetch()} disabled={q.isFetching}>
          <RefreshCw className={cn("size-3.5", q.isFetching && "animate-spin")} /> Refresh
        </Button>
        <span className="text-xs text-muted-foreground">Snapshot of the last {tail} lines per container; refresh to update.</span>
      </div>
      {q.error ? (
        <p className="text-destructive">Could not load logs: {(q.error as Error).message}</p>
      ) : (
        <ComposeLogList rows={lines} loading={q.isLoading} loadedAt={q.dataUpdatedAt} />
      )}
    </div>
  );
}
