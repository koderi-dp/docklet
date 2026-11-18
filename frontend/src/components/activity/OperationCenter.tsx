import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Bell, ChevronDown, ChevronRight } from "lucide-react";
import { OperationStatus, type Operation } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { useUI } from "@/stores/uiStore";
import { AlertsPanel } from "@/components/notifications/AlertsPanel";
import { useConnections } from "@/hooks/useActiveConnection";
import { useAlertEvents, useAlerts } from "@/hooks/useNotifications";
import { useOperationLogs, useOperations } from "@/hooks/useOperations";
import { RegistryAuthOffer } from "@/components/registries/RegistryAuthOffer";
import { StatusIcon } from "./OperationStatusIcon";
import { OperationHistory } from "./OperationHistory";

function formatElapsed(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}

function Elapsed({ op }: { op: Operation }) {
  const running = op.status === OperationStatus.OpRunning;
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!running) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [running]);
  const end = running ? now : op.finishedAt || now;
  return <span className="tabular-nums">{formatElapsed(end - op.startedAt)}</span>;
}

function ProgressBar({ op }: { op: Operation }) {
  if (op.status !== OperationStatus.OpRunning) return null;
  const known = op.progress != null;
  return (
    <div
      className="h-1 w-full overflow-hidden rounded-full bg-muted"
      role="progressbar"
      aria-label={known ? "Progress" : "Running, progress unknown"}
      aria-valuemin={known ? 0 : undefined}
      aria-valuemax={known ? 100 : undefined}
      aria-valuenow={known ? Math.round((op.progress as number) * 100) : undefined}
    >
      {known ? (
        <div className="h-full rounded-full bg-primary transition-[width]" style={{ width: `${Math.round((op.progress as number) * 100)}%` }} />
      ) : (
        <div className="h-full w-full animate-pulse rounded-full bg-primary/40" />
      )}
    </div>
  );
}

function LogView({ id }: { id: string }) {
  const { lines } = useOperationLogs(id, true);
  const ref = useRef<HTMLDivElement>(null);
  const stick = useRef(true);
  useLayoutEffect(() => {
    const el = ref.current;
    if (el && stick.current) el.scrollTop = el.scrollHeight;
  }, [lines]);
  return (
    <div
      ref={ref}
      onScroll={(e) => {
        const el = e.currentTarget;
        stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
      }}
      className="mt-2 max-h-48 overflow-auto rounded-md border border-border bg-muted/40 p-2 font-mono text-[11px] leading-snug"
      tabIndex={0}
      aria-label="Operation log"
    >
      {lines.length === 0 ? (
        <span className="text-muted-foreground">No log output yet.</span>
      ) : (
        lines.map((l, i) => (
          <div key={i} className="whitespace-pre-wrap break-all">
            {l}
          </div>
        ))
      )}
    </div>
  );
}

function OperationRow({ op, connectionName, onCancel }: { op: Operation; connectionName: string; onCancel: () => void }) {
  const [expanded, setExpanded] = useState(false);
  const running = op.status === OperationStatus.OpRunning;
  return (
    <li className="border-b border-border/60 px-3 py-2.5 last:border-b-0">
      <div className="flex items-start gap-2">
        <div className="pt-0.5">
          <StatusIcon status={op.status} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-[13px] font-medium" title={op.title}>
              {op.title}
            </span>
            {running && op.cancellable && (
              <Button size="sm" variant="outline" className="h-6 px-2 text-xs" onClick={onCancel}>
                Cancel
              </Button>
            )}
          </div>
          <div className="mt-0.5 flex items-center gap-2 text-xs text-muted-foreground">
            <span className="truncate" title={connectionName}>
              {connectionName}
            </span>
            <span aria-hidden>·</span>
            <Elapsed op={op} />
            {op.status === OperationStatus.OpCancelled && <span>Cancelled</span>}
            {running && op.progress != null && <span className="tabular-nums">{Math.round(op.progress * 100)}%</span>}
          </div>
          <div className="mt-1.5">
            <ProgressBar op={op} />
          </div>
          {op.status === OperationStatus.OpFailed && op.error ? (
            <p className="mt-1.5 break-words text-xs text-destructive">{op.error}</p>
          ) : (
            op.detail && (
              <p className="mt-1 truncate text-xs text-muted-foreground" title={op.detail}>
                {op.detail}
              </p>
            )
          )}
          <RegistryAuthOffer op={op} />
          <button
            type="button"
            className="mt-1 inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
            aria-expanded={expanded}
            onClick={() => setExpanded((v) => !v)}
          >
            {expanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
            Logs
          </button>
          {expanded && <LogView id={op.id} />}
        </div>
      </div>
    </li>
  );
}

/** Top-bar activity center: bell with running-operation and unread-alert counts; Operations and Alerts tabs. */
export default function OperationCenter() {
  const { operations, runningCount, finishedCount, loaded, loadError, centerOpen, setCenterOpen, cancel, clearFinished } = useOperations();
  const { data: conns } = useConnections();
  const { unreadCount } = useAlerts();
  const activityTab = useUI((s) => s.activityTab);
  const setActivityTab = useUI((s) => s.setActivityTab);
  useAlertEvents();
  const nameOf = (id: string) => conns?.find((c) => c.id === id)?.name ?? id;
  const label = [runningCount > 0 && `${runningCount} running`, unreadCount > 0 && `${unreadCount} unread alerts`].filter(Boolean).join(", ");
  return (
    <Popover open={centerOpen} onOpenChange={setCenterOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" className="relative" title="Activity and alerts" aria-label={label ? `Activity: ${label}` : "Activity"}>
          <Bell className="size-4" />
          {runningCount > 0 && (
            <Badge className="absolute -right-0.5 -top-0.5 h-4 min-w-4 justify-center rounded-full px-1 text-[10px] leading-none">
              {runningCount}
            </Badge>
          )}
          {unreadCount > 0 && (
            <Badge className="absolute -bottom-0.5 -right-0.5 h-4 min-w-4 justify-center rounded-full bg-destructive px-1 text-[10px] leading-none text-white">
              {unreadCount > 9 ? "9+" : unreadCount}
            </Badge>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[420px] p-0">
        <Tabs value={activityTab} onValueChange={(v) => setActivityTab(v as "operations" | "alerts")} className="gap-0">
          <TabsList variant="line" className="w-full justify-start border-b border-border px-2">
            <TabsTrigger value="operations">Operations</TabsTrigger>
            <TabsTrigger value="alerts">Alerts{unreadCount > 0 ? ` (${unreadCount})` : ""}</TabsTrigger>
          </TabsList>
          <TabsContent value="operations">
            <div className="flex items-center justify-end border-b border-border px-3 py-1.5">
              <Button size="sm" variant="ghost" className="h-7 text-xs" disabled={finishedCount === 0} onClick={() => void clearFinished()}>
                Clear finished
              </Button>
            </div>
            <div className="max-h-[360px] overflow-auto">
              {loadError ? (
                <p className="p-4 text-sm text-destructive">Could not load operations: {loadError}</p>
              ) : !loaded ? (
                <p className="p-4 text-sm text-muted-foreground">Loading…</p>
              ) : operations.length === 0 ? (
                <p className="p-4 text-sm text-muted-foreground">No background operations. Pulls, prunes and other long tasks appear here.</p>
              ) : (
                <ul>
                  {operations.map((op) => (
                    <OperationRow key={op.id} op={op} connectionName={nameOf(op.connectionId)} onCancel={() => void cancel(op.id)} />
                  ))}
                </ul>
              )}
            </div>
            <OperationHistory open={centerOpen} liveIds={new Set(operations.map((o) => o.id))} nameOf={nameOf} />
          </TabsContent>
          <TabsContent value="alerts">
            <AlertsPanel />
          </TabsContent>
        </Tabs>
      </PopoverContent>
    </Popover>
  );
}
