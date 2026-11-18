import { useUI } from "@/stores/uiStore";
import { useCallback, useDeferredValue, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ArrowDownToLine, Copy, Download, Eraser, Pause, Play, RefreshCw, Search } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { MAX_LOG_LINES, useLogStream, type ViewLine } from "./useLogStream";

const TAIL_OPTIONS: Array<{ label: string; value: number }> = [
  { label: "100 lines", value: 100 },
  { label: "500 lines", value: 500 },
  { label: "1000 lines", value: 1000 },
  { label: "5000 lines", value: 5000 },
  { label: "All", value: -1 },
];

// eslint-disable-next-line no-control-regex
const ANSI_RE = /\u001b\[[0-9;?]*[ -/]*[@-~]/g;
const stripAnsi = (s: string) => s.replace(ANSI_RE, "");

function formatLine(l: ViewLine, timestamps: boolean): string {
  const text = stripAnsi(l.text);
  return timestamps && l.timestamp ? `${l.timestamp} ${text}` : text;
}

interface Props {
  connectionId: string;
  containerId: string;
  containerName: string;
  /** Smaller toolbar + fixed height for the inspector panel. */
  compact?: boolean;
  className?: string;
}

/**
 * Streaming log viewer. Buffer capped at MAX_LOG_LINES; follow stops when the user scrolls up.
 * The backend stream is stopped on unmount (tab change / navigation).
 */
export function LogViewer({ connectionId, containerId, containerName, compact, className }: Props) {
  const [tail, setTail] = useState(() => (compact ? 100 : useUI.getState().logTail));
  const [timestamps, setTimestamps] = useState(true);
  const [search, setSearch] = useState("");
  const [follow, setFollow] = useState(true);
  const { lines, status, error, paused, setPaused, queued, clear, reconnect } = useLogStream({
    connectionId,
    containerId,
    tail,
    timestamps,
  });
  const scroller = useRef<HTMLDivElement>(null);

  const deferredSearch = useDeferredValue(search);
  const needle = deferredSearch.trim().toLowerCase();
  const shown = useMemo(
    () => (needle ? lines.filter((l) => stripAnsi(l.text).toLowerCase().includes(needle)) : lines),
    [lines, needle],
  );

  // Stick to the bottom while following.
  useLayoutEffect(() => {
    const el = scroller.current;
    if (follow && el) el.scrollTop = el.scrollHeight;
  }, [shown, follow]);

  const onScroll = useCallback(() => {
    const el = scroller.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
    setFollow((f) => (f === atBottom ? f : atBottom));
  }, []);

  // Wheel up always releases follow immediately (before the scroll event settles).
  useEffect(() => {
    const el = scroller.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      if (e.deltaY < 0) setFollow(false);
    };
    el.addEventListener("wheel", onWheel, { passive: true });
    return () => el.removeEventListener("wheel", onWheel);
  }, []);

  const resume = () => {
    setFollow(true);
    const el = scroller.current;
    if (el) el.scrollTop = el.scrollHeight;
  };

  const exportText = () => shown.map((l) => formatLine(l, timestamps)).join("\n");

  const copyAll = () => {
    navigator.clipboard
      .writeText(exportText())
      .then(() => toast.success(`Copied ${shown.length} line${shown.length === 1 ? "" : "s"}`))
      .catch((e: unknown) => toast.error("Could not copy logs", { description: String(e) }));
  };

  const download = () => {
    const blob = new Blob([exportText() + "\n"], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${containerName || containerId.slice(0, 12)}.log`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  const statusText =
    status === "connecting"
      ? "Connecting…"
      : status === "live"
        ? paused
          ? `Paused${queued ? ` · ${queued} new` : ""}`
          : "Live"
        : status === "ended"
          ? "Stream ended"
          : "Stream error";

  return (
    <div className={cn("flex min-h-0 flex-col gap-2", className)}>
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[140px] flex-1">
          <Search className="pointer-events-none absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter lines"
            aria-label="Filter log lines"
            className="h-8 pl-7 text-[13px]"
          />
        </div>
        <select
          aria-label="Tail lines"
          value={tail}
          onChange={(e) => setTail(Number(e.target.value))}
          className="h-8 rounded-md border border-border bg-card px-2 text-[13px] text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
        >
          {TAIL_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
        <label className="flex cursor-pointer items-center gap-1.5 text-[13px]">
          <input type="checkbox" checked={timestamps} onChange={(e) => setTimestamps(e.target.checked)} className="accent-primary" />
          Timestamps
        </label>
        <Button size="sm" variant="outline" onClick={() => setPaused(!paused)} disabled={status !== "live"}>
          {paused ? <Play className="size-3.5" /> : <Pause className="size-3.5" />}
          {paused ? "Resume" : "Pause"}
        </Button>
        {!compact && (
          <>
            <Button size="sm" variant="outline" onClick={copyAll} disabled={shown.length === 0}>
              <Copy className="size-3.5" /> Copy
            </Button>
            <Button size="sm" variant="outline" onClick={download} disabled={shown.length === 0}>
              <Download className="size-3.5" /> Download
            </Button>
          </>
        )}
        <Button size="sm" variant="outline" onClick={clear} disabled={lines.length === 0} title="Clears this view only; the container's logs are not touched">
          <Eraser className="size-3.5" /> Clear view
        </Button>
      </div>

      <div className="relative min-h-0 flex-1">
        <div
          ref={scroller}
          onScroll={onScroll}
          role="log"
          aria-label={`Logs of ${containerName}`}
          className="h-full min-h-[160px] overflow-auto rounded-md border border-border bg-muted/30 p-2 font-mono text-[11.5px] leading-[1.45]"
        >
          {shown.length === 0 ? (
            <p className="p-2 text-muted-foreground">
              {status === "connecting"
                ? "Connecting to log stream…"
                : needle
                  ? "No lines match the filter."
                  : "No log output yet."}
            </p>
          ) : (
            shown.map((l) => (
              <div
                key={l.seq}
                className={cn("whitespace-pre-wrap break-all", l.stream === "stderr" ? "text-warning" : "text-foreground")}
              >
                {timestamps && l.timestamp && <span className="mr-2 text-muted-foreground">{l.timestamp}</span>}
                {stripAnsi(l.text)}
              </div>
            ))
          )}
        </div>
        {!follow && shown.length > 0 && (
          <Button size="sm" className="absolute bottom-3 right-4 shadow-sm" onClick={resume}>
            <ArrowDownToLine className="size-3.5" /> Resume follow
          </Button>
        )}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>
          {statusText}
          {needle ? ` · ${shown.length} of ${lines.length} lines match` : ` · ${lines.length} lines`}
          {lines.length >= MAX_LOG_LINES && ` (view keeps the latest ${MAX_LOG_LINES})`}
          {follow ? " · following" : ""}
        </span>
        {(status === "ended" || status === "error") && (
          <span className="flex items-center gap-2">
            {error && <span className="text-destructive">{error}</span>}
            <Button size="sm" variant="outline" className="h-6 px-2 text-xs" onClick={reconnect}>
              <RefreshCw className="size-3" /> Reconnect
            </Button>
          </span>
        )}
      </div>
    </div>
  );
}
