import { ArrowDown, ArrowUp } from "lucide-react";
import { ariaSort } from "@/lib/sorting";
import { useTableSort } from "@/lib/useTableSort";
import { cn } from "@/lib/utils";

/** Sortable column header cell: a button with an arrow indicator and aria-sort on the <th>. */
export function SortTh({
  table,
  id,
  children,
  className,
  align,
}: {
  table: string;
  id: string;
  children: React.ReactNode;
  className?: string;
  align?: "right";
}) {
  const { sort, toggle } = useTableSort(table);
  const active = sort?.id === id;
  return (
    <th scope="col" aria-sort={ariaSort(sort, id)} className={cn("h-10 whitespace-nowrap px-3 font-medium", align === "right" && "text-right", className)}>
      <button
        type="button"
        onClick={() => toggle(id)}
        className={cn(
          "-mx-1 inline-flex items-center gap-1 rounded px-1 py-0.5 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none",
          active && "text-foreground",
        )}
      >
        {children}
        {active && sort?.desc ? <ArrowDown className="size-3" aria-hidden /> : <ArrowUp className={cn("size-3", !active && "opacity-0")} aria-hidden />}
      </button>
    </th>
  );
}
