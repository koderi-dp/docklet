import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { OperationStatus, type Operation } from "@bindings/model";
import { usePastOperations } from "@/hooks/useOperations";
import { StatusIcon } from "./OperationStatusIcon";

/**
 * Collapsed "History" section of the Activity popover: metadata of past operations only (no logs).
 * Loads only while the popover is open and the section expanded; skips operations already shown as live.
 */
export function OperationHistory({ open, liveIds, nameOf }: { open: boolean; liveIds: ReadonlySet<string>; nameOf: (connectionId: string) => string }) {
  const [expanded, setExpanded] = useState(false);
  const past = usePastOperations(open && expanded, 50);
  const items = (past.data ?? []).filter((op: Operation) => !liveIds.has(op.id));

  return (
    <section className="border-t border-border">
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-center gap-1.5 px-3 py-2 text-left text-xs font-medium text-muted-foreground transition-colors duration-100 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none"
      >
        {expanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
        History
      </button>
      {expanded && (
        <div className="max-h-64 overflow-auto">
          {past.isLoading ? (
            <p className="px-3 pb-3 text-xs text-muted-foreground">Loading…</p>
          ) : past.error ? (
            <p className="px-3 pb-3 text-xs text-destructive">Could not load history: {String(past.error)}</p>
          ) : items.length === 0 ? (
            <p className="px-3 pb-3 text-xs text-muted-foreground">No earlier operations.</p>
          ) : (
            <ul>
              {items.map((op: Operation) => (
                <li key={op.id} className="flex items-start gap-2 border-t border-border/60 px-3 py-2">
                  <div className="pt-0.5"><StatusIcon status={op.status} /></div>
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-[13px]" title={op.title}>{op.title}</div>
                    <div className="mt-0.5 flex items-center gap-2 text-xs text-muted-foreground">
                      <span className="truncate" title={nameOf(op.connectionId)}>{nameOf(op.connectionId)}</span>
                      <span aria-hidden>·</span>
                      <span className="shrink-0 tabular-nums">{op.finishedAt ? new Date(op.finishedAt).toLocaleString() : "—"}</span>
                      {op.status === OperationStatus.OpCancelled && <span>Cancelled</span>}
                    </div>
                    {op.status === OperationStatus.OpFailed && op.error && (
                      <p className="mt-1 break-words text-xs text-destructive">{op.error}</p>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}
