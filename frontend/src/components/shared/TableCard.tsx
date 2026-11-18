import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/** Bordered card that holds a toolbar (or selection bar) plus a table. Also a size container for toolbar queries. */
export function TableCard({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("@container overflow-hidden rounded-[10px] border border-border bg-card", className)}>{children}</div>;
}

/**
 * Fixed-height (48px) single-line toolbar row. When `selection` is given (a <SelectionBar/>), it REPLACES
 * the toolbar content in the same row so nothing shifts.
 */
export function TableToolbar({
  children,
  selection,
  className,
}: {
  children?: ReactNode;
  selection?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex h-12 flex-nowrap items-center gap-2 border-b border-border px-3", selection && "bg-accent/40", className)}>
      {selection ?? children}
    </div>
  );
}
