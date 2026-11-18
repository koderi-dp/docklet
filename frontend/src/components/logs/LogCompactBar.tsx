import type { ReactNode } from "react";
import { Clock, Copy, Download, Eraser, MoreHorizontal, Pause, Play, WrapText } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { IconButton } from "@/components/shared/IconButton";
import { LOG_LEVELS, type LogLevel } from "@/lib/logLevel";
import { TIME_RANGES, type TimeRange } from "@/lib/logRange";
import { LEVEL_DOT, LEVEL_LABEL } from "./logColours";
import { cn } from "@/lib/utils";
import { LogSearchBox } from "./LogSearchBox";
import type { LogFilterState } from "./useLogFilter";
import type { LogSearchState } from "./useLogSearch";

const SHOWN_LEVELS = LOG_LEVELS.filter((l) => l !== "other");

interface Props {
  search: LogSearchState;
  filter: LogFilterState;
  levelCounts: Record<LogLevel, number>;
  paused: boolean;
  onPause: () => void;
  canPause: boolean;
  wrap: boolean;
  onWrap: (v: boolean) => void;
  timestamps: boolean;
  onTimestamps: (v: boolean) => void;
  hasVisible: boolean;
  hasLines: boolean;
  onCopy: () => void;
  onDownload: () => void;
  onClear: () => void;
  /** Items at the top of the overflow menu (the number of lines to load). */
  menuExtra?: ReactNode;
}

/**
 * One control row for the narrow inspector panel: search, then icon buttons, then an overflow menu with
 * the less used things (levels, time range, copy, download, clear). Filter rules are only managed in the full view.
 */
export function LogCompactBar(p: Props) {
  const f = p.filter;
  return (
    <div className="flex items-center gap-1">
      <LogSearchBox search={p.search} />
      <IconButton
        variant={p.paused ? "secondary" : "ghost"}
        label={p.paused ? "Resume" : "Pause"}
        tooltip={p.canPause ? (p.paused ? "Show new lines again" : "Freeze the view; new lines keep arriving in the background") : "Not streaming"}
        disabled={!p.canPause}
        onClick={p.onPause}
      >
        {p.paused ? <Play className="size-4" /> : <Pause className="size-4" />}
      </IconButton>
      <IconButton variant={p.wrap ? "secondary" : "ghost"} label="Wrap long lines" aria-pressed={p.wrap} onClick={() => p.onWrap(!p.wrap)}>
        <WrapText className="size-4" />
      </IconButton>
      <IconButton variant={p.timestamps ? "secondary" : "ghost"} label="Show time" tooltip="Show the time of each line" aria-pressed={p.timestamps} onClick={() => p.onTimestamps(!p.timestamps)}>
        <Clock className="size-4" />
      </IconButton>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant={f.active ? "secondary" : "ghost"} size="icon-sm" className="size-8 shrink-0" aria-label="More log options" title="More: lines to load, levels, time range, copy, download, clear">
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          {p.menuExtra}
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>Levels · {f.levels.size === 0 ? "all" : [...f.levels].map((l) => LEVEL_LABEL[l]).join(", ")}</DropdownMenuSubTrigger>
            <DropdownMenuSubContent className="w-44">
              {SHOWN_LEVELS.map((l) => (
                <DropdownMenuCheckboxItem key={l} checked={f.levels.has(l)} onCheckedChange={() => f.toggleLevel(l)} onSelect={(e) => e.preventDefault()}>
                  <span aria-hidden className={cn("size-1.5 rounded-full", LEVEL_DOT[l])} />
                  {LEVEL_LABEL[l]}
                  <span className="ml-auto text-xs text-muted-foreground tabular-nums">{p.levelCounts[l].toLocaleString()}</span>
                </DropdownMenuCheckboxItem>
              ))}
            </DropdownMenuSubContent>
          </DropdownMenuSub>
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>Time range · {TIME_RANGES.find((r) => r.id === f.range)?.label ?? f.range}</DropdownMenuSubTrigger>
            <DropdownMenuSubContent>
              <DropdownMenuRadioGroup value={f.range} onValueChange={(v) => f.setRange(v as TimeRange)}>
                {TIME_RANGES.map((r) => (
                  <DropdownMenuRadioItem key={r.id} value={r.id}>
                    {r.label}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuSubContent>
          </DropdownMenuSub>
          <DropdownMenuSeparator />
          <DropdownMenuItem disabled={!p.hasVisible} onSelect={p.onCopy}>
            <Copy /> Copy visible
          </DropdownMenuItem>
          <DropdownMenuItem disabled={!p.hasVisible} onSelect={p.onDownload}>
            <Download /> Download
          </DropdownMenuItem>
          <DropdownMenuItem disabled={!p.hasLines} onSelect={p.onClear}>
            <Eraser /> Clear view
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
