import { useLayoutEffect, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { AnsiText } from "@/components/logs/AnsiText";
import type { ComposeLogRow } from "@/lib/composeLogs";
import { cn } from "@/lib/utils";

const NO_MARKS: never[] = [];
const ROW_ESTIMATE = 20;

function time(ts: string): string {
  const d = new Date(ts);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleTimeString(undefined, { hour12: false }) + "." + String(d.getMilliseconds()).padStart(3, "0");
}

interface Props {
  rows: readonly ComposeLogRow[];
  loading: boolean;
  /** Changes on every load or refresh; the list then jumps to the newest line. */
  loadedAt: number;
}

/** The Compose Logs tab's scroll box: only the visible rows are in the DOM, rows wrap and are measured as drawn. */
export function ComposeLogList({ rows, loading, loadedAt }: Props) {
  const box = useRef<HTMLDivElement>(null);
  // Stays at the newest line until the user scrolls away from it.
  const pinned = useRef(true);

  const v = useVirtualizer({
    count: rows.length,
    getScrollElement: () => box.current,
    estimateSize: () => ROW_ESTIMATE,
    overscan: 20,
    getItemKey: (i) => rows[i].seq,
  });
  const total = v.getTotalSize();

  useLayoutEffect(() => {
    pinned.current = true;
  }, [rows, loadedAt]);

  // Runs again as rows are measured and the total height settles.
  useLayoutEffect(() => {
    const el = box.current;
    if (pinned.current && el) el.scrollTop = el.scrollHeight;
  }, [rows, loadedAt, total]);

  const onScroll = () => {
    const el = box.current;
    if (el) pinned.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
  };

  return (
    <div ref={box} onScroll={onScroll} className="min-h-[200px] flex-1 overflow-auto rounded-[10px] border border-border bg-card p-3 font-mono text-xs leading-5">
      {loading ? (
        <span className="text-muted-foreground">Loading…</span>
      ) : rows.length === 0 ? (
        <span className="text-muted-foreground">No log output.</span>
      ) : (
        <div className="relative w-full" style={{ height: total }}>
          {v.getVirtualItems().map((item) => {
            const l = rows[item.index];
            return (
              <div
                key={item.key}
                data-index={item.index}
                ref={v.measureElement}
                className="absolute left-0 top-0 flex w-full gap-2 whitespace-pre-wrap break-all"
                style={{ transform: `translateY(${item.start}px)` }}
              >
                <span className="shrink-0 text-muted-foreground">{time(l.timestamp)}</span>
                <span className="w-24 shrink-0 truncate text-info" title={l.container}>
                  {l.service || l.container}
                </span>
                <span className={cn(l.stream === "stderr" && "text-warning")}>{l.spans ? <AnsiText text={l.text} spans={l.spans} ranges={NO_MARKS} current={false} /> : l.text}</span>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
