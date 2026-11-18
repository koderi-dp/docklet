import { TIME_RANGES, type TimeRange } from "@/lib/logRange";
import type { LogLevel } from "@/lib/logLevel";
import { cn } from "@/lib/utils";
import { FilterGroup, FilterPopover } from "@/components/shared/FilterPopover";
import type { useLogView } from "./useLogView";

type View = ReturnType<typeof useLogView>;

const LEVEL_LABEL: Record<LogLevel, string> = { error: "Error", warn: "Warn", info: "Info", debug: "Debug", other: "Other" };
const LEVEL_DOT: Record<LogLevel, string> = {
  error: "bg-destructive",
  warn: "bg-warning",
  info: "bg-info",
  debug: "bg-muted-foreground/60",
  other: "bg-border",
};

export const selectClass =
  "h-8 rounded-md border border-border bg-card px-2 text-[13px] text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring/60";

/** Level toggle chips with live counts. */
export function LevelChips({ view, className }: { view: View; className?: string }) {
  return (
    <div className={cn("flex items-center gap-1.5 text-xs", className)} role="group" aria-label="Log levels">
      {view.levelOrder.map((l) => {
        const on = view.levels.has(l);
        return (
          <button
            key={l}
            type="button"
            aria-pressed={on}
            onClick={() => view.toggleLevel(l)}
            className={cn(
              "inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md border px-2 transition-colors focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none",
              on ? "border-primary/50 bg-primary/10 text-foreground" : "border-border text-muted-foreground hover:bg-muted/60",
            )}
          >
            <span aria-hidden className={cn("size-1.5 rounded-full", LEVEL_DOT[l])} />
            {LEVEL_LABEL[l]}
            <span className="min-w-[1ch] tabular-nums">{view.counts[l]}</span>
          </button>
        );
      })}
    </div>
  );
}

export function TimeRangeSelect({ view, className }: { view: View; className?: string }) {
  return (
    <select aria-label="Time range" value={view.range} onChange={(e) => view.setRange(e.target.value as TimeRange)} className={cn(selectClass, className)}>
      {TIME_RANGES.map((r) => (
        <option key={r.id} value={r.id}>
          {r.label}
        </option>
      ))}
    </select>
  );
}

/** Narrow layouts: the level chips and time range collapse into the shared Filters popover. */
export function LogFilterPopover({ view }: { view: View }) {
  const active = view.levels.size + (view.range === "all" ? 0 : 1);
  return (
    <FilterPopover
      activeCount={active}
      onClear={() => {
        for (const l of [...view.levels]) view.toggleLevel(l);
        view.setRange("all");
      }}
    >
      <div>
        <div className="mb-1.5 text-xs font-medium text-muted-foreground">Level</div>
        <LevelChips view={view} className="flex-wrap" />
      </div>
      <FilterGroup label="Time range" value={view.range} options={TIME_RANGES.map((r) => ({ value: r.id, label: r.label }))} onChange={view.setRange} />
    </FilterPopover>
  );
}
