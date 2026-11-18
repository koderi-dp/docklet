import { ChevronDown, ChevronUp, Copy, Download, Eraser, Pause, Play, Regex, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { IconButton } from "@/components/shared/IconButton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { LevelChips, LogFilterPopover, selectClass, TimeRangeSelect } from "./LogLevelFilters";
import type { useLogView } from "./useLogView";

export const TAIL_OPTIONS: Array<{ label: string; value: number }> = [
  { label: "100 lines", value: 100 },
  { label: "500 lines", value: 500 },
  { label: "1000 lines", value: 1000 },
  { label: "5000 lines", value: 5000 },
  { label: "All", value: -1 },
];

type View = ReturnType<typeof useLogView>;

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
    <div className="@container space-y-2">
      <div className={cn("flex items-center gap-2", p.compact && "flex-wrap")}>
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
        {/* Wide: chips and range inline; narrow: both inside the Filters popover. */}
        <LevelChips view={v} className="hidden @[1020px]:flex" />
        <TimeRangeSelect view={v} className="hidden @[1020px]:block" />
        <div className="@[1020px]:hidden">
          <LogFilterPopover view={v} />
        </div>
        <select aria-label="Tail lines" value={p.tail} onChange={(e) => p.onTail(Number(e.target.value))} className={cn(selectClass, "shrink-0")}>
          {TAIL_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
        <label className="flex shrink-0 cursor-pointer items-center gap-1.5 text-[13px]">
          <Checkbox checked={p.timestamps} onCheckedChange={(c) => p.onTimestamps(c === true)} />
          Timestamps
        </label>
        <Button size="sm" variant="outline" className="shrink-0" onClick={p.onPause} disabled={!p.canPause}>
          {p.paused ? <Play className="size-3.5" /> : <Pause className="size-3.5" />}
          {p.paused ? "Resume" : "Pause"}
        </Button>
        {!p.compact && (
          <>
            <IconButton variant="outline" label="Copy visible" tooltip="Copy the lines currently shown (after filters)" onClick={p.onCopy} disabled={p.visibleCount === 0}>
              <Copy className="size-3.5" />
            </IconButton>
            <IconButton variant="outline" label="Download" tooltip="Download the lines currently shown" onClick={p.onDownload} disabled={p.visibleCount === 0}>
              <Download className="size-3.5" />
            </IconButton>
          </>
        )}
        <IconButton variant="outline" label="Clear view" tooltip="Clears this view only; the container's logs are not touched" onClick={p.onClear} disabled={!p.hasLines}>
          <Eraser className="size-3.5" />
        </IconButton>
      </div>

      {v.regexError && (
        <p role="alert" className="text-xs text-destructive">
          Invalid regular expression: {v.regexError}. No lines are filtered.
        </p>
      )}
    </div>
  );
}
