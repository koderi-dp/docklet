import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { LOG_LEVELS, type LogLevel } from "@/lib/logLevel";
import { TIME_RANGES } from "@/lib/logRange";
import type { EvalResult } from "@/lib/logFilter/rules";
import { LEVEL_DOT, LEVEL_LABEL } from "./logColours";
import { AddFilterPopover } from "./AddFilterPopover";
import { LogSearchBox } from "./LogSearchBox";
import { RuleChip } from "./RuleChip";
import { SavedFilters } from "./SavedFilters";
import type { LogFilterState } from "./useLogFilter";
import type { LogSearchState } from "./useLogSearch";

const SHOWN_LEVELS = LOG_LEVELS.filter((l) => l !== "other");
const RANGE_LABEL: Record<string, string> = { "5m": "5 min", "15m": "15 min", "1h": "1 h", all: "All" };

function LevelChips({ f, counts }: { f: LogFilterState; counts: Record<LogLevel, number> }) {
  return (
    <div className="flex shrink-0 items-center gap-1" role="group" aria-label="Log levels">
      {SHOWN_LEVELS.map((l) => {
        const on = f.levels.has(l);
        return (
          <button
            key={l}
            type="button"
            aria-pressed={on}
            title={on ? `Showing ${LEVEL_LABEL[l]} lines. Click to stop filtering by this level.` : counts[l] === 0 ? `No ${LEVEL_LABEL[l]} lines in what the filters currently show` : `Show only ${LEVEL_LABEL[l]} lines (several levels can be on)`}
            onClick={() => f.toggleLevel(l)}
            className={cn(
              "inline-flex h-8 items-center gap-1.5 rounded-md border px-2 text-xs transition-colors focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none",
              on ? "border-primary/50 bg-primary/10 text-foreground" : "border-border text-muted-foreground hover:bg-muted/60",
              !on && counts[l] === 0 && "opacity-50",
            )}
          >
            <span aria-hidden className={cn("size-1.5 rounded-full", LEVEL_DOT[l])} />
            {LEVEL_LABEL[l]}
            <span className="tabular-nums">{counts[l].toLocaleString()}</span>
          </button>
        );
      })}
    </div>
  );
}

function TimeChips({ f }: { f: LogFilterState }) {
  return (
    <div className="flex shrink-0 items-center overflow-hidden rounded-md border border-border" role="group" aria-label="Time range">
      {TIME_RANGES.map((r) => (
        <button
          key={r.id}
          type="button"
          aria-pressed={f.range === r.id}
          onClick={() => f.setRange(r.id)}
          className={cn(
            "h-[30px] border-r border-border px-2 text-xs last:border-r-0 focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none focus-visible:ring-inset",
            f.range === r.id ? "bg-primary/10 text-foreground" : "text-muted-foreground hover:bg-muted/60",
          )}
        >
          {RANGE_LABEL[r.id]}
        </button>
      ))}
    </div>
  );
}

function IncludeMode({ f }: { f: LogFilterState }) {
  return (
    <div className="flex h-7 shrink-0 items-center gap-1.5 text-xs text-muted-foreground" role="group" aria-label="How include rules combine">
      Match
      <div className="flex overflow-hidden rounded-md border border-border">
        {(["any", "all"] as const).map((m) => (
          <button
            key={m}
            type="button"
            aria-pressed={f.includeMode === m}
            onClick={() => f.setIncludeMode(m)}
            title={m === "any" ? "A line is kept when ANY include rule matches" : "A line is kept only when ALL include rules match"}
            className={cn("h-[26px] px-2", f.includeMode === m ? "bg-primary/10 text-foreground" : "hover:bg-muted/60")}
          >
            {m === "any" ? "any" : "all"}
          </button>
        ))}
      </div>
      include
    </div>
  );
}

interface Props {
  filter: LogFilterState;
  search: LogSearchState;
  result: Pick<EvalResult<unknown>, "counts" | "levelCounts">;
  /** Containers whose lines are shown (suggestions for a Container filter); empty hides that target. */
  containerNames?: readonly string[];
  /** Right-aligned controls on the level / time row (the action buttons). */
  trailing?: ReactNode;
  className?: string;
}

/**
 * The filter bar. Row 1: search (finds, never hides) on the left, explicit filters on the right ("+ Filter",
 * saved filters). Row 2: level and time chips and the action buttons. Row 3: the filter chips.
 * Used by the Logs screen and the full-page single-container viewer.
 */
export function LogFilterBar({ filter: f, search, result, containerNames, trailing, className }: Props) {
  const includes = f.rules.filter((r) => r.kind === "include" && r.enabled).length;
  const hasChips = f.rules.length > 0;
  const keep = () => {
    f.addRule({ kind: "include", target: "message", pattern: search.query });
    search.clear();
  };
  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex items-center gap-1.5">
        <LogSearchBox search={search} onKeep={keep} />
        <div className="flex shrink-0 items-center gap-1.5">
          <AddFilterPopover rules={f.rules} addRule={f.addRule} containerNames={containerNames} />
          <SavedFilters f={f} />
        </div>
      </div>
      {search.error && (
        <p role="alert" className="text-xs text-destructive">
          Invalid regular expression: {search.error}. Nothing is marked.
        </p>
      )}
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1.5">
        <div className="flex flex-wrap items-center gap-2">
          <LevelChips f={f} counts={result.levelCounts} />
          <TimeChips f={f} />
        </div>
        {trailing}
      </div>

      {!hasChips && <p className="text-xs text-muted-foreground">No filters. Use + Filter, or right-click a log line.</p>}

      {hasChips && (
        <div className="flex max-h-[84px] flex-wrap items-center gap-1.5 overflow-y-auto">
          {includes >= 2 && <IncludeMode f={f} />}
          {f.rules.map((r) => (
            <RuleChip key={r.id} rule={r} count={result.counts[r.id]} rules={f.rules} containerNames={containerNames} onChange={(patch) => f.updateRule(r.id, patch)} onRemove={() => f.removeRule(r.id)} />
          ))}
          <button type="button" onClick={f.clearAll} className="h-7 rounded px-1.5 text-xs text-muted-foreground hover:bg-muted/60 hover:text-foreground" title="Remove all filters and reset levels and time range">
            Clear all
          </button>
        </div>
      )}
    </div>
  );
}
