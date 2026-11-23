import type { ProjectFilter } from "@/lib/projectRowModel";
import { cn } from "@/lib/utils";

const CHIPS: { value: ProjectFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "running", label: "Running" },
  { value: "stopped", label: "Stopped" },
];

export function ProjectFilterChips({
  counts,
  value,
  onChange,
}: {
  counts: Record<ProjectFilter, number>;
  value: ProjectFilter;
  onChange: (f: ProjectFilter) => void;
}) {
  return (
    <div role="group" aria-label="Filter projects" className="flex items-center gap-1.5">
      {CHIPS.map((c) => {
        const selected = value === c.value;
        return (
          <button
            key={c.value}
            type="button"
            aria-pressed={selected}
            onClick={() => onChange(c.value)}
            className={cn(
              "inline-flex h-7 items-center gap-1.5 rounded-full border px-2.5 text-xs transition-colors hover:bg-accent/60",
              selected ? "border-primary/40 bg-accent text-accent-foreground" : "border-border",
            )}
          >
            {c.label} <span className="tabular-nums text-muted-foreground">{counts[c.value]}</span>
          </button>
        );
      })}
    </div>
  );
}
