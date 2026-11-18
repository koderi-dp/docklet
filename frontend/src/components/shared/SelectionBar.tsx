import type { ReactNode } from "react";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";

/**
 * Inline selection bar rendered INSIDE <TableToolbar selection={...}/> in place of the toolbar:
 * "N selected" + bulk actions (children, destructive last) + muted note + Clear on the right.
 */
export function SelectionBar({
  count,
  note,
  onClear,
  children,
}: {
  count: number;
  /** Muted text, e.g. "2 hidden by filters". */
  note?: ReactNode;
  onClear: () => void;
  children?: ReactNode;
}) {
  return (
    <>
      <span className="shrink-0 whitespace-nowrap text-[13px] font-medium" aria-live="polite">
        {count} selected
      </span>
      <div className="flex shrink-0 items-center gap-1.5">{children}</div>
      {note ? <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">{note}</span> : <span className="flex-1" />}
      <Button size="sm" variant="ghost" className="h-8 shrink-0 text-muted-foreground" onClick={onClear}>
        <X className="size-3.5" /> Clear
      </Button>
    </>
  );
}
