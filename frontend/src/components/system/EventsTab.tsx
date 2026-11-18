import { useMemo, useState } from "react";
import { Activity, RefreshCw } from "lucide-react";
import type { EngineEvent } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/EmptyState";
import { useRecentEvents } from "@/hooks/useSystem";
import { cn } from "@/lib/utils";
import { Card, formatEventTime } from "./parts";

const WINDOWS = [
  { label: "Last 15 minutes", seconds: 900 },
  { label: "Last hour", seconds: 3600 },
  { label: "Last 6 hours", seconds: 6 * 3600 },
  { label: "Last 24 hours", seconds: 24 * 3600 },
];

const LIMIT = 200;

/** Compact attribute summary (image, exit code ...), never the full label set. */
export function attrSummary(e: EngineEvent): string {
  const a = e.attributes ?? {};
  const parts: string[] = [];
  if (a.image) parts.push(a.image);
  if (a.container) parts.push(`container ${a.container.slice(0, 12)}`);
  if (a.exitCode) parts.push(`exit ${a.exitCode}`);
  if (a.signal) parts.push(`signal ${a.signal}`);
  if (a["com.docker.compose.project"]) parts.push(`project ${a["com.docker.compose.project"]}`);
  return parts.join(" · ");
}

export function EventsTab({ connectionId }: { connectionId: string }) {
  const [seconds, setSeconds] = useState(3600);
  const [type, setType] = useState("all");
  const [auto, setAuto] = useState(true);
  const q = useRecentEvents(connectionId, seconds, LIMIT, auto);

  const events = q.data ?? [];
  const types = useMemo(() => ["all", ...[...new Set(events.map((e) => e.type))].sort()], [events]);
  const shown = type === "all" ? events : events.filter((e) => e.type === type);

  return (
    <Card
      title="Recent Engine events"
      subtitle={`Latest ${LIMIT} events of the selected window, newest first${auto ? " · refreshes every 10 s while this page is visible" : ""}`}
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <Select value={String(seconds)} onValueChange={(v) => setSeconds(Number(v))}>
            <SelectTrigger size="sm" className="h-8 w-[150px]" aria-label="Time window"><SelectValue /></SelectTrigger>
            <SelectContent>
              {WINDOWS.map((w) => <SelectItem key={w.seconds} value={String(w.seconds)}>{w.label}</SelectItem>)}
            </SelectContent>
          </Select>
          <Select value={type} onValueChange={setType}>
            <SelectTrigger size="sm" className="h-8 w-[130px]" aria-label="Event type"><SelectValue /></SelectTrigger>
            <SelectContent>
              {types.map((t) => <SelectItem key={t} value={t}>{t === "all" ? "All types" : t}</SelectItem>)}
            </SelectContent>
          </Select>
          <label className="flex items-center gap-2 text-[13px]">
            <Checkbox checked={auto} onCheckedChange={(c) => setAuto(c === true)} /> Auto-refresh
          </label>
          <Button size="sm" variant="outline" onClick={() => void q.refetch()} disabled={q.isFetching}>
            <RefreshCw className={cn("size-3.5", q.isFetching && "animate-spin")} /> Refresh
          </Button>
        </div>
      }
    >
      {q.isLoading ? (
        <Skeleton className="h-60 w-full" />
      ) : q.error ? (
        <EmptyState icon={<Activity />} title="Could not load events">
          <p className="text-destructive">{String(q.error)}</p>
          <Button className="mt-3" onClick={() => void q.refetch()}>Retry</Button>
        </EmptyState>
      ) : shown.length === 0 ? (
        <EmptyState icon={<Activity />} title="No events in this window">
          <p>The Engine reported no {type === "all" ? "" : `${type} `}events for the selected period.</p>
        </EmptyState>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[640px] border-collapse text-[13px]">
            <thead>
              <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
                <th className="h-9 px-2 font-medium">Time</th>
                <th className="px-2 font-medium">Type</th>
                <th className="px-2 font-medium">Action</th>
                <th className="px-2 font-medium">Resource</th>
                <th className="px-2 font-medium">Details</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((e, i) => (
                <tr key={`${e.time}-${e.actorId}-${i}`} className="h-[38px] border-b border-border/60">
                  <td className="whitespace-nowrap px-2 tabular-nums text-muted-foreground" title={new Date(e.time).toLocaleString()}>{formatEventTime(e.time)}</td>
                  <td className="px-2">{e.type}</td>
                  <td className="max-w-[220px] px-2"><span className="block truncate" title={e.action}>{e.action}</span></td>
                  <td className="max-w-[240px] px-2"><span className="block truncate font-medium" title={e.actorId}>{e.actorName}</span></td>
                  <td className="max-w-[320px] px-2"><span className="block truncate text-muted-foreground" title={attrSummary(e)}>{attrSummary(e) || "—"}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}
