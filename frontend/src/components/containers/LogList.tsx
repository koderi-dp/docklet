import type { RefObject } from "react";
import { segments } from "@/lib/logSearch";
import { cn } from "@/lib/utils";
import type { ShownLine } from "./useLogView";

const LEVEL_TINT = {
  error: "text-destructive",
  warn: "text-warning",
  info: "text-foreground",
  debug: "text-muted-foreground",
  other: "text-foreground",
} as const;

/** Level colour; stderr lines without a recognisable level stay tinted as before. */
function tint(l: ShownLine): string {
  if (l.level === "other" && l.line.stream === "stderr") return "text-warning";
  return LEVEL_TINT[l.level];
}

function LineText({ l, active }: { l: ShownLine; active: number }) {
  if (l.ranges.length === 0) return <>{l.text}</>;
  return (
    <>
      {segments(l.text, l.ranges).map((s, i) =>
        s.match ? (
          <mark
            key={i}
            data-match={l.firstMatch + (s.index ?? 0)}
            className={cn("rounded-[2px] px-px text-foreground", l.firstMatch + (s.index ?? 0) === active ? "bg-primary/50 ring-1 ring-primary" : "bg-warning/30")}
          >
            {s.text}
          </mark>
        ) : (
          <span key={i}>{s.text}</span>
        ),
      )}
    </>
  );
}

export function LogList({
  shown,
  timestamps,
  active,
  emptyText,
  containerName,
  scroller,
  onScroll,
}: {
  shown: readonly ShownLine[];
  timestamps: boolean;
  active: number;
  emptyText: string;
  containerName: string;
  scroller: RefObject<HTMLDivElement | null>;
  onScroll: () => void;
}) {
  return (
    <div
      ref={scroller}
      onScroll={onScroll}
      role="log"
      aria-label={`Logs of ${containerName}`}
      className="h-full min-h-[160px] overflow-auto rounded-md border border-border bg-muted/30 p-2 font-mono text-[11.5px] leading-[1.45]"
    >
      {shown.length === 0 ? (
        <p className="p-2 text-muted-foreground">{emptyText}</p>
      ) : (
        shown.map((l) => (
          <div key={l.line.seq} className={cn("whitespace-pre-wrap break-all", tint(l))}>
            {timestamps && l.line.timestamp && <span className="mr-2 text-muted-foreground">{l.line.timestamp}</span>}
            <LineText l={l} active={active} />
          </div>
        ))
      )}
    </div>
  );
}
