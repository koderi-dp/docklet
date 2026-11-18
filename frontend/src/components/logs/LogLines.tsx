import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ContextMenu, ContextMenuTrigger } from "@/components/ui/context-menu";
import type { LogEntry } from "@/lib/logEntry";
import type { HighlightColour } from "@/lib/logFilter/rules";
import { LineMenu, type LineActions, type LineContext } from "./LineMenu";
import { LogRow } from "./LogRow";

interface Props {
  entries: readonly LogEntry[];
  highlights: ReadonlyArray<HighlightColour | undefined>;
  showContainer: boolean;
  tagWidth: number;
  timestamps: boolean;
  wrap: boolean;
  /** Stick to the newest line. The list reports false when the user scrolls up and true when back at the bottom. */
  follow: boolean;
  onFollowChange: (follow: boolean) => void;
  actions: LineActions;
  ariaLabel: string;
  /** Shown instead of the list when there is nothing to show. */
  empty?: ReactNode;
}

const ROW_ESTIMATE = 18;

/** Virtualised log list: only the visible rows are in the DOM, however many lines are loaded. */
export function LogLines({ entries, highlights, showContainer, tagWidth, timestamps, wrap, follow, onFollowChange, actions, ariaLabel, empty }: Props) {
  const scroller = useRef<HTMLDivElement>(null);
  const lastTop = useRef(0);
  const [ctx, setCtx] = useState<LineContext | null>(null);

  const v = useVirtualizer({
    count: entries.length,
    getScrollElement: () => scroller.current,
    estimateSize: () => ROW_ESTIMATE,
    overscan: 24,
    getItemKey: (i) => entries[i].seq,
  });
  const total = v.getTotalSize();

  // Wrapping changes every row height.
  useEffect(() => v.measure(), [wrap, v]);

  useLayoutEffect(() => {
    const el = scroller.current;
    if (follow && el) el.scrollTop = el.scrollHeight;
  }, [follow, entries, total]);

  const onScroll = useCallback(() => {
    const el = scroller.current;
    if (!el) return;
    const top = el.scrollTop;
    if (el.scrollHeight - top - el.clientHeight < 24) onFollowChange(true);
    else if (top < lastTop.current - 2) onFollowChange(false);
    lastTop.current = top;
  }, [onFollowChange]);

  // Wheel up releases follow at once, before the scroll event settles.
  useEffect(() => {
    const el = scroller.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      if (e.deltaY < 0) onFollowChange(false);
    };
    el.addEventListener("wheel", onWheel, { passive: true });
    return () => el.removeEventListener("wheel", onWheel);
  }, [onFollowChange]);

  const captureContext = (e: React.MouseEvent) => {
    const row = (e.target as HTMLElement).closest<HTMLElement>("[data-index]");
    const entry = row ? entries[Number(row.dataset.index)] : undefined;
    const sel = window.getSelection();
    const inside = !!sel && sel.rangeCount > 0 && !!scroller.current?.contains(sel.anchorNode);
    setCtx(entry ? { entry, selection: inside ? sel.toString().trim() : "" } : null);
  };

  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>
        <div
          ref={scroller}
          role="log"
          aria-label={ariaLabel}
          onScroll={onScroll}
          onContextMenu={captureContext}
          className="h-full min-h-[160px] overflow-auto rounded-md border border-border bg-muted/30 font-mono text-[11.5px] leading-[1.45]"
        >
          {entries.length === 0 ? (
            (empty ?? null)
          ) : (
            <div className="relative w-full" style={{ height: total }}>
              {v.getVirtualItems().map((item) => (
                <div key={item.key} data-index={item.index} ref={v.measureElement} className="absolute left-0 top-0 w-full" style={{ transform: `translateY(${item.start}px)` }}>
                  <LogRow entry={entries[item.index]} highlight={highlights[item.index]} showContainer={showContainer} timestamps={timestamps} wrap={wrap} tagWidth={tagWidth} />
                </div>
              ))}
            </div>
          )}
        </div>
      </ContextMenuTrigger>
      <LineMenu ctx={ctx} actions={actions} showContainer={showContainer} />
    </ContextMenu>
  );
}
