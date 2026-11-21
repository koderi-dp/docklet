import { useEffect, useMemo, useRef, useState } from "react";
import { RefreshCw } from "lucide-react";
import { AnsiText } from "@/components/logs/AnsiText";
import { Button } from "@/components/ui/button";
import { useComposeLogs } from "@/hooks/useCompose";
import { parseAnsi } from "@/lib/ansi";
import { cn } from "@/lib/utils";

const TAILS = [200, 500, 1000, 2000];
const selectCls =
  "h-8 rounded-md border border-border bg-background px-2 text-[13px] text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring/60";

const NO_MARKS: never[] = [];

function time(ts: string): string {
  const d = new Date(ts);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleTimeString(undefined, { hour12: false }) + "." + String(d.getMilliseconds()).padStart(3, "0");
}

/** Merged, timestamp-ordered snapshot of the project's container logs (not a live stream). */
export function LogsTab({ connectionId, project, services, active }: { connectionId: string; project: string; services: string[]; active: boolean }) {
  const [service, setService] = useState("all");
  const [tail, setTail] = useState(500);
  const q = useComposeLogs(connectionId, project, tail, active);
  const box = useRef<HTMLDivElement>(null);
  const lines = useMemo(() => (q.data ?? []).filter((l) => service === "all" || l.service === service).map((l) => ({ ...l, ...parseAnsi(l.text) })), [q.data, service]);

  useEffect(() => {
    if (box.current) box.current.scrollTop = box.current.scrollHeight;
  }, [lines]);

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
        <div ref={box} className="min-h-[200px] flex-1 overflow-auto rounded-[10px] border border-border bg-card p-3 font-mono text-xs leading-5">
          {q.isLoading ? (
            <span className="text-muted-foreground">Loading…</span>
          ) : lines.length === 0 ? (
            <span className="text-muted-foreground">No log output.</span>
          ) : (
            lines.map((l, i) => (
              <div key={i} className="flex gap-2 whitespace-pre-wrap break-all">
                <span className="shrink-0 text-muted-foreground">{time(l.timestamp)}</span>
                <span className="w-24 shrink-0 truncate text-info" title={l.container}>
                  {l.service || l.container}
                </span>
                <span className={cn(l.stream === "stderr" && "text-warning")}>{l.spans ? <AnsiText text={l.text} spans={l.spans} ranges={NO_MARKS} current={false} /> : l.text}</span>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
}
