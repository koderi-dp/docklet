import { memo } from "react";
import { cn } from "@/lib/utils";
import { formatLogTime, type LogEntry } from "@/lib/logEntry";
import type { HighlightColour } from "@/lib/logFilter/rules";
import { containerTone, HIGHLIGHT_BG, LEVEL_TEXT } from "./logColours";

/** Level colour; stderr lines without a recognisable level keep the warning tint of the old viewer. */
function tint(e: LogEntry): string {
  return e.level === "other" && e.stream === "stderr" ? "text-warning" : LEVEL_TEXT[e.level];
}

interface Props {
  entry: LogEntry;
  highlight?: HighlightColour;
  showContainer: boolean;
  timestamps: boolean;
  wrap: boolean;
  /** Width of the container tag column, in characters. */
  tagWidth: number;
}

export const LogRow = memo(function LogRow({ entry, highlight, showContainer, timestamps, wrap, tagWidth }: Props) {
  return (
    <div className={cn("flex gap-2 px-2 py-px", wrap ? "w-full" : "w-max min-w-full", highlight && HIGHLIGHT_BG[highlight])}>
      {timestamps && entry.timestamp && (
        <span className="shrink-0 text-muted-foreground tabular-nums" title={entry.timestamp}>
          {formatLogTime(entry.time)}
        </span>
      )}
      {showContainer && (
        <span className={cn("shrink-0 truncate font-medium", containerTone(entry.container).text)} style={{ width: `${tagWidth}ch` }} title={entry.container}>
          {entry.container}
        </span>
      )}
      <span className={cn("min-w-0 flex-1", wrap ? "whitespace-pre-wrap [overflow-wrap:anywhere]" : "whitespace-pre", tint(entry))}>{entry.text}</span>
    </div>
  );
});
