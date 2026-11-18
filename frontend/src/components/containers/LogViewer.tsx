import { useMemo, useState, type ReactNode } from "react";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem } from "@/components/ui/dropdown-menu";
import { useUI } from "@/stores/uiStore";
import { makeEntry, type LogEntry } from "@/lib/logEntry";
import { sortByTime } from "@/lib/logMerge";
import { LogView } from "@/components/logs/LogView";
import { useFilteredLogs } from "@/components/logs/useFilteredLogs";
import { useLogFilter } from "@/components/logs/useLogFilter";
import { MAX_LOG_LINES, useLogStream, type ViewLine } from "./useLogStream";

const TAIL_OPTIONS: Array<{ label: string; value: number }> = [
  { label: "100 lines", value: 100 },
  { label: "500 lines", value: 500 },
  { label: "1000 lines", value: 1000 },
  { label: "5000 lines", value: 5000 },
  { label: "All", value: -1 },
];

// A line never changes after it arrives, so its text and level are computed once.
const entryCache = new WeakMap<ViewLine, LogEntry>();
function toEntry(line: ViewLine, container: string, fallbackKey: number): LogEntry {
  let e = entryCache.get(line);
  if (!e) {
    e = makeEntry(line, container, line.seq, fallbackKey);
    entryCache.set(line, e);
  }
  return e;
}

interface Props {
  connectionId: string;
  containerId: string;
  containerName: string;
  /** Smaller layout + fixed height for the inspector panel. */
  compact?: boolean;
  className?: string;
  /** Right end of the status row (the inspector puts its link to the full logs here). */
  statusEnd?: ReactNode;
}

/**
 * Streaming log viewer of one container. Same filter bar and list as the Logs screen; the buffer is capped at
 * MAX_LOG_LINES and the backend stream is stopped on unmount (tab change / navigation).
 */
export function LogViewer({ connectionId, containerId, containerName, compact, className, statusEnd }: Props) {
  const [tail, setTail] = useState(() => (compact ? 100 : useUI.getState().logTail));
  // The stream always carries timestamps (the time-range filter needs them); the Time toggle only hides them.
  const { lines, status, error, paused, setPaused, queued, clear, reconnect } = useLogStream({ connectionId, containerId, tail, timestamps: true });
  // The Engine delivers stdout and stderr separately, so arrival order is not time order: sort by timestamp.
  const entries = useMemo(() => {
    let key = 0;
    const list = lines.map((l) => {
      const e = toEntry(l, containerName, key);
      key = e.sortKey;
      return e;
    });
    return sortByTime(list) as LogEntry[];
  }, [lines, containerName]);
  const filter = useLogFilter({ restore: false });
  const result = useFilteredLogs(entries, filter);

  const stateText =
    status === "connecting" ? "Connecting…" : status === "live" ? (paused ? `Paused${queued ? ` · ${queued} new` : ""}` : "Live") : status === "ended" ? "Stream ended" : "Stream error";

  const statusNode = (
    <span className="flex flex-wrap items-center gap-x-1.5">
      <span>
        {stateText}
        {` · ${result.visible.length.toLocaleString()} of ${entries.length.toLocaleString()} lines`}
        {entries.length >= MAX_LOG_LINES && ` (keeps the latest ${MAX_LOG_LINES.toLocaleString()})`}
      </span>
      {(status === "ended" || status === "error") && (
        <>
          {error && <span className="text-destructive">{error}</span>}
          <Button size="sm" variant="outline" className="h-6 px-2 text-xs" onClick={reconnect}>
            <RefreshCw className="size-3" /> Reconnect
          </Button>
        </>
      )}
    </span>
  );

  const tailMenu = (
    <>
      <DropdownMenuLabel className="text-xs text-muted-foreground">Load on open</DropdownMenuLabel>
      <DropdownMenuRadioGroup value={String(tail)} onValueChange={(v) => setTail(Number(v))}>
        {TAIL_OPTIONS.map((o) => (
          <DropdownMenuRadioItem key={o.value} value={String(o.value)}>
            {o.label}
          </DropdownMenuRadioItem>
        ))}
      </DropdownMenuRadioGroup>
    </>
  );

  const tailSelect = (
    <select
      aria-label="Tail lines"
      value={tail}
      onChange={(e) => setTail(Number(e.target.value))}
      className="h-8 shrink-0 rounded-md border border-border bg-card px-2 text-[13px] text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
    >
      {TAIL_OPTIONS.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );

  return (
    <LogView
      className={className}
      total={entries.length}
      filter={filter}
      result={result}
      showContainer={false}
      tagWidth={0}
      status={statusNode}
      extra={compact ? undefined : tailSelect}
      compact={compact}
      menuExtra={compact ? tailMenu : undefined}
      statusEnd={statusEnd}
      paused={paused}
      onPause={() => setPaused(!paused)}
      canPause={status === "live"}
      onClear={clear}
      emptyNoLines={<p className="p-3 text-muted-foreground">{status === "connecting" ? "Connecting to log stream…" : "No log output yet."}</p>}
      ariaLabel={`Logs of ${containerName}`}
      downloadName={containerName || containerId.slice(0, 12)}
    />
  );
}
