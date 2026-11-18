import type { ReactNode } from "react";
import { ListFilter } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";

/** Single "Filters" button with a count badge; content = FilterGroup sections; Clear lives inside. */
export function FilterPopover({
  activeCount,
  onClear,
  children,
}: {
  activeCount: number;
  onClear: () => void;
  children: ReactNode;
}) {
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          size="sm"
          variant="outline"
          className="h-8 shrink-0 gap-1.5"
          aria-label={activeCount ? `Filters (${activeCount} active)` : "Filters"}
          title="Filters"
        >
          <ListFilter className="size-3.5" />
          <span className="hidden @[640px]:inline">Filters</span>
          {activeCount > 0 && (
            <span className="rounded-full bg-primary px-1.5 text-[11px] leading-4 text-primary-foreground tabular-nums">{activeCount}</span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-72 space-y-3 p-3">
        {children}
        <div className="flex justify-end border-t border-border pt-2">
          <Button size="sm" variant="ghost" className="h-7" disabled={activeCount === 0} onClick={onClear}>
            Clear filters
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}

/** Labelled group of mutually exclusive option chips. */
export function FilterGroup<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div>
      <div className="mb-1.5 text-xs font-medium text-muted-foreground">{label}</div>
      <div className="flex flex-wrap gap-1.5" role="group" aria-label={label}>
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            aria-pressed={value === o.value}
            onClick={() => onChange(o.value)}
            className={cn(
              "h-7 rounded-md border px-2 text-xs capitalize outline-none transition-colors duration-100 focus-visible:ring-2 focus-visible:ring-ring/60",
              value === o.value
                ? "border-primary bg-primary/15 text-foreground"
                : "border-border text-muted-foreground hover:bg-muted hover:text-foreground",
            )}
          >
            {o.label}
          </button>
        ))}
      </div>
    </div>
  );
}
