import { cn } from "@/lib/utils";
import type { RowCounts, ServiceFilter } from "@/lib/serviceRows";

const CHIPS: { value: ServiceFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "running", label: "Running" },
  { value: "stopped", label: "Stopped" },
];

/** Small segmented control over the Services table; every count equals the rows it shows. */
export function ServiceFilterChips({ value, counts, onChange }: { value: ServiceFilter; counts: RowCounts; onChange: (f: ServiceFilter) => void }) {
  return (
    <div role="group" aria-label="Filter services" className="inline-flex rounded-md border border-border bg-card p-0.5">
      {CHIPS.map((chip) => (
        <button
          key={chip.value}
          type="button"
          aria-pressed={value === chip.value}
          onClick={() => onChange(chip.value)}
          className={cn(
            "inline-flex h-6 items-center gap-1.5 rounded px-2.5 text-xs font-medium text-muted-foreground hover:text-foreground",
            value === chip.value && "bg-accent text-foreground",
          )}
        >
          {chip.value === "running" && <span aria-hidden className="size-1.5 rounded-full bg-success" />}
          {chip.label}
          <span className="tabular-nums">{counts[chip.value]}</span>
        </button>
      ))}
    </div>
  );
}
