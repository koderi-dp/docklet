import type { ReactNode } from "react";
import { Plus, Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { LOG_LEVELS, type LogLevel } from "@/lib/logLevel";
import { TIME_RANGES } from "@/lib/logRange";
import type { EvalResult, RuleKind, RuleTarget } from "@/lib/logFilter/rules";
import { LEVEL_DOT, LEVEL_LABEL } from "./logColours";
import { KIND_LABEL, RuleChip, TARGET_LABEL } from "./RuleChip";
import { SavedFilters } from "./SavedFilters";
import type { LogFilterState } from "./useLogFilter";

const SHOWN_LEVELS = LOG_LEVELS.filter((l) => l !== "other");
const RANGE_LABEL: Record<string, string> = { "5m": "5 min", "15m": "15 min", "1h": "1 h", all: "All" };

const PLACEHOLDER: Record<RuleTarget, string> = {
  message: "Filter lines: text, GET /api/*, /timeout|refused/",
  container: "Container name: web-*, db",
  level: "Level: error, warn, info, debug",
};

const selectTrigger = "h-8 w-auto shrink-0 gap-1.5 px-2 text-[13px]";

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
            onClick={() => f.toggleLevel(l)}
            className={cn(
              "inline-flex h-8 items-center gap-1.5 rounded-md border px-2 text-xs transition-colors focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none",
              on ? "border-primary/50 bg-primary/10 text-foreground" : "border-border text-muted-foreground hover:bg-muted/60",
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
  result: Pick<EvalResult<unknown>, "counts" | "levelCounts">;
  /** Right-aligned controls on the level / time row (the action buttons). */
  trailing?: ReactNode;
  className?: string;
}

/**
 * The one filter bar: a live input (Enter turns it into a rule chip), level and time chips, saved filters,
 * and the stack of rule chips. Used by the Logs screen and the single-container viewer.
 */
export function LogFilterBar({ filter: f, result, trailing, className }: Props) {
  const includes = f.rules.filter((r) => r.kind === "include" && r.enabled).length;
  const hasChips = f.rules.length > 0;
  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex items-center gap-1.5">
        <div className="flex min-w-0 flex-1 items-center gap-1.5">
          <Select value={f.draftKind} onValueChange={(v) => f.setDraftKind(v as RuleKind)}>
            <SelectTrigger size="sm" className={selectTrigger} aria-label="Rule kind">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(Object.keys(KIND_LABEL) as RuleKind[]).map((k) => (
                <SelectItem key={k} value={k}>
                  {KIND_LABEL[k]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={f.draftTarget} onValueChange={(v) => f.setDraftTarget(v as RuleTarget)}>
            <SelectTrigger size="sm" className={selectTrigger} aria-label="Rule applies to">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(Object.keys(TARGET_LABEL) as RuleTarget[]).map((t) => (
                <SelectItem key={t} value={t}>
                  {TARGET_LABEL[t]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={f.text}
              onChange={(e) => f.setText(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  f.commitDraft();
                } else if (e.key === "Escape") f.setText("");
              }}
              placeholder={PLACEHOLDER[f.draftTarget]}
              aria-label="Filter log lines"
              aria-invalid={!!f.liveError}
              className={cn("h-8 pr-8 pl-7 text-[13px]", f.liveError && "border-destructive")}
            />
            {f.text !== "" && (
              <div className="absolute top-1/2 right-1 flex -translate-y-1/2 items-center gap-1">
                <button type="button" aria-label="Clear text" title="Clear (Esc)" onClick={() => f.setText("")} className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground">
                  <X className="size-3" />
                </button>
              </div>
            )}
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-8 shrink-0 gap-1.5 px-2.5 text-[13px]"
            disabled={f.text.trim() === "" || !!f.liveError}
            title={f.liveError ? "Fix the pattern first" : f.text.trim() === "" ? "Type what to filter on first" : "Keep this as a filter (Enter)"}
            onClick={f.commitDraft}
          >
            <Plus className="size-3.5" /> Add filter
          </Button>
        </div>
        <SavedFilters f={f} />
      </div>
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1.5">
        <div className="flex flex-wrap items-center gap-2">
          <LevelChips f={f} counts={result.levelCounts} />
          <TimeChips f={f} />
        </div>
        {trailing}
      </div>

      {f.liveError && (
        <p role="alert" className="text-xs text-destructive">
          Invalid regular expression: {f.liveError}. Not applied yet.
        </p>
      )}

      {!hasChips && (
        <p className="text-xs text-muted-foreground">
          No filters yet. Type above and press Enter or Add filter to keep it, or right-click a log line and choose Include, Exclude or Highlight. Filters stack.
        </p>
      )}

      {hasChips && (
        <div className="flex max-h-[84px] flex-wrap items-center gap-1.5 overflow-y-auto">
          {includes >= 2 && <IncludeMode f={f} />}
          {f.rules.map((r) => (
            <RuleChip key={r.id} rule={r} count={result.counts[r.id]} onChange={(patch) => f.updateRule(r.id, patch)} onRemove={() => f.removeRule(r.id)} />
          ))}
        </div>
      )}
    </div>
  );
}
