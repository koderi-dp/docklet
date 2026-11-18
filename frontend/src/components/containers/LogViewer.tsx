import { useUI } from "@/stores/uiStore";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { ArrowDownToLine, RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { LogList } from "./LogList";
import { LogToolbar } from "./LogToolbar";
import { MAX_LOG_LINES, useLogStream } from "./useLogStream";
import { useLogView, type ShownLine } from "./useLogView";

function formatLine(l: ShownLine, timestamps: boolean): string {
  return timestamps && l.line.timestamp ? `${l.line.timestamp} ${l.text}` : l.text;
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
  const [follow, setFollow] = useState(true);
  // The stream always carries timestamps (the time-range filter needs them); the toggle only hides them.
  const { lines, status, error, paused, setPaused, queued, clear, reconnect } = useLogStream({
    connectionId,
    containerId,
    tail,
    timestamps: true,
  });
  const view = useLogView(lines);
  const { shown, totalMatches, active } = view;
  const scroller = useRef<HTMLDivElement>(null);
  const navigated = useRef(false);

  // Stick to the bottom while following.
  useLayoutEffect(() => {
    const el = scroller.current;
    if (follow && el) el.scrollTop = el.scrollHeight;
  }, [shown, follow]);

  // After next / previous match: bring that match into view and stop following.
  useEffect(() => {
    if (!navigated.current) return;
    navigated.current = false;
    scroller.current?.querySelector(`[data-match="${active}"]`)?.scrollIntoView({ block: "center" });
  }, [active, shown]);

  const step = useCallback(
    (dir: 1 | -1) => {
      navigated.current = true;
      setTimeout(() => (navigated.current = false), 300); // a single match does not change `active`
      setFollow(false);
      view.step(dir);
    },
    [view],
  );

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

  const copyVisible = () => {
    navigator.clipboard
      .writeText(exportText())
      .then(() => toast.success(`Copied ${shown.length} visible line${shown.length === 1 ? "" : "s"}`))
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

  const filtered = shown.length !== lines.length;
  const emptyText =
    status === "connecting" ? "Connecting to log stream…" : lines.length > 0 ? "No lines match the filters." : "No log output yet.";

  return (
    <div className={cn("flex min-h-0 flex-col gap-2", className)}>
      <LogToolbar
        view={view}
        tail={tail}
        onTail={setTail}
        timestamps={timestamps}
        onTimestamps={setTimestamps}
        paused={paused}
        canPause={status === "live"}
        onPause={() => setPaused(!paused)}
        compact={compact}
        visibleCount={shown.length}
        hasLines={lines.length > 0}
        onCopy={copyVisible}
        onDownload={download}
        onClear={clear}
        onStep={step}
      />

      <div className="relative min-h-0 flex-1">
        <LogList shown={shown} timestamps={timestamps} active={active} emptyText={emptyText} containerName={containerName} scroller={scroller} onScroll={onScroll} />
        {!follow && shown.length > 0 && (
          <Button size="sm" className="absolute bottom-3 right-4 shadow-sm" onClick={resume}>
            <ArrowDownToLine className="size-3.5" /> Resume follow
          </Button>
        )}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>
          {statusText}
          {filtered ? ` · ${shown.length} of ${lines.length} lines` : ` · ${lines.length} lines`}
          {view.hasQuery && ` · ${totalMatches === 0 ? "no matches" : `${totalMatches} match${totalMatches === 1 ? "" : "es"} (${active + 1} of ${totalMatches})`}`}
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
