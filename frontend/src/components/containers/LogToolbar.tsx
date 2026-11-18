import { ChevronDown, ChevronUp, Copy, Download, Eraser, Pause, Play, Regex, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { TIME_RANGES, type TimeRange } from "@/lib/logRange";
import type { LogLevel } from "@/lib/logLevel";
import { cn } from "@/lib/utils";
import type { useLogView } from "./useLogView";

export const TAIL_OPTIONS: Array<{ label: string; value: number }> = [
  { label: "100 lines", value: 100 },
  { label: "500 lines", value: 500 },
  { label: "1000 lines", value: 1000 },
  { label: "5000 lines", value: 5000 },
  { label: "All", value: -1 },
];

const LEVEL_LABEL: Record<LogLevel, string> = { error: "Error", warn: "Warn", info: "Info", debug: "Debug", other: "Other" };
const LEVEL_DOT: Record<LogLevel, string> = {
  error: "bg-destructive",
  warn: "bg-warning",
  info: "bg-info",
  debug: "bg-muted-foreground/60",
  other: "bg-border",
};

type View = ReturnType<typeof useLogView>;

const selectClass =
  "h-8 rounded-md border border-border bg-card px-2 text-[13px] text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring/60";

export interface LogToolbarProps {
  view: View;
  tail: number;
  onTail: (n: number) => void;
  timestamps: boolean;
  onTimestamps: (v: boolean) => void;
  paused: boolean;
  canPause: boolean;
  onPause: () => void;
  compact?: boolean;
  visibleCount: number;
  hasLines: boolean;
  onCopy: () => void;
  onDownload: () => void;
  onClear: () => void;
  onStep: (dir: 1 | -1) => void;
}

/** Search (text / regex, Enter and Shift+Enter step through matches), level chips, time range and actions. */
export function LogToolbar(p: LogToolbarProps) {
  const v = p.view;
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[160px] flex-1">
          <Search className="pointer-events-none absolute left-2 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={v.search}
            onChange={(e) => v.setSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                p.onStep(e.shiftKey ? -1 : 1);
              }
            }}
            placeholder={v.useRegex ? "Regular expression" : "Filter lines"}
            aria-label="Filter log lines"
            aria-invalid={!!v.regexError}
            className={cn("h-8 pl-7 pr-[74px] text-[13px]", v.useRegex && "font-mono", v.regexError && "border-destructive")}
          />
          <div className="absolute right-1 top-1/2 flex -translate-y-1/2 items-center">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant={v.useRegex ? "secondary" : "ghost"}
                  size="icon"
                  className="size-6"
                  aria-label="Use regular expression"
                  aria-pressed={v.useRegex}
                  onClick={() => v.setUseRegex(!v.useRegex)}
                >
                  <Regex className="size-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Regular expression (case-insensitive)</TooltipContent>
            </Tooltip>
            <Button type="button" variant="ghost" size="icon" className="size-6" aria-label="Previous match (Shift+Enter)" title="Previous match (Shift+Enter)" disabled={v.totalMatches === 0} onClick={() => p.onStep(-1)}>
              <ChevronUp className="size-3.5" />
            </Button>
            <Button type="button" variant="ghost" size="icon" className="size-6" aria-label="Next match (Enter)" title="Next match (Enter)" disabled={v.totalMatches === 0} onClick={() => p.onStep(1)}>
              <ChevronDown className="size-3.5" />
            </Button>
          </div>
        </div>
        <select aria-label="Tail lines" value={p.tail} onChange={(e) => p.onTail(Number(e.target.value))} className={selectClass}>
          {TAIL_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
        <label className="flex cursor-pointer items-center gap-1.5 text-[13px]">
          <input type="checkbox" checked={p.timestamps} onChange={(e) => p.onTimestamps(e.target.checked)} className="accent-primary" />
          Timestamps
        </label>
        <Button size="sm" variant="outline" onClick={p.onPause} disabled={!p.canPause}>
          {p.paused ? <Play className="size-3.5" /> : <Pause className="size-3.5" />}
          {p.paused ? "Resume" : "Pause"}
        </Button>
        {!p.compact && (
          <>
            <Button size="sm" variant="outline" onClick={p.onCopy} disabled={p.visibleCount === 0} title="Copies the lines currently shown (after filters)">
              <Copy className="size-3.5" /> Copy visible
            </Button>
            <Button size="sm" variant="outline" onClick={p.onDownload} disabled={p.visibleCount === 0}>
              <Download className="size-3.5" /> Download
            </Button>
          </>
        )}
        <Button size="sm" variant="outline" onClick={p.onClear} disabled={!p.hasLines} title="Clears this view only; the container's logs are not touched">
          <Eraser className="size-3.5" /> Clear view
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-1.5 text-xs">
        {v.levelOrder.map((l) => {
          const on = v.levels.has(l);
          return (
            <button
              key={l}
              type="button"
              aria-pressed={on}
              onClick={() => v.toggleLevel(l)}
              className={cn(
                "inline-flex h-6 items-center gap-1.5 rounded-md border px-2 transition-colors focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none",
                on ? "border-primary/50 bg-primary/10 text-foreground" : "border-border text-muted-foreground hover:bg-muted/60",
              )}
            >
              <span aria-hidden className={cn("size-1.5 rounded-full", LEVEL_DOT[l])} />
              {LEVEL_LABEL[l]}
              <span className="tabular-nums">{v.counts[l]}</span>
            </button>
          );
        })}
        <select aria-label="Time range" value={v.range} onChange={(e) => v.setRange(e.target.value as TimeRange)} className={cn(selectClass, "ml-auto h-6 text-xs")}>
          {TIME_RANGES.map((r) => (
            <option key={r.id} value={r.id}>
              {r.label}
            </option>
          ))}
        </select>
      </div>
      {v.regexError && (
        <p role="alert" className="text-xs text-destructive">
          Invalid regular expression: {v.regexError}. No lines are filtered.
        </p>
      )}
    </div>
  );
}
