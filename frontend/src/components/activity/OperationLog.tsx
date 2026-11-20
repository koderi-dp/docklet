import { useLayoutEffect, useRef } from "react";

/** Log of one operation, filling the space it is given and following new output until the user scrolls up. */
export function OperationLog({ lines }: { lines: string[] }) {
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
      className="min-h-0 flex-1 overflow-auto rounded-md border border-border bg-muted/40 p-3 font-mono text-xs leading-relaxed focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none"
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
