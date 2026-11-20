import { memo, useMemo } from "react";
import { cn } from "@/lib/utils";
import { formatLogClock, formatLogTime, type LogEntry } from "@/lib/logEntry";
import type { HighlightColour } from "@/lib/logFilter/rules";
import type { Search } from "@/lib/logFilter/search";
import { containerTone, HIGHLIGHT_BG, LEVEL_TEXT } from "./logColours";
import { AnsiText } from "./AnsiText";
import { MarkedText } from "./MarkedText";

/** Colour of the text the program did not colour itself: the level colour; stderr lines without a recognisable level keep the warning tint of the old viewer. */
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
  /** Time as HH:mm:ss (the compact viewer); the full value stays in the tooltip. */
  shortTime?: boolean;
  /** Matches of the search are marked in the message. */
  search?: Search | null;
  /** This line holds the current match. */
  current?: boolean;
}

export const LogRow = memo(function LogRow({ entry, highlight, showContainer, timestamps, wrap, tagWidth, shortTime, search, current }: Props) {
  const ranges = useMemo(() => (search ? search.ranges(entry.text) : []), [search, entry.text]);
  return (
    <div className={cn("flex gap-2 px-2 py-px", wrap ? "w-full" : "w-max min-w-full", highlight && HIGHLIGHT_BG[highlight])}>
      {timestamps && entry.timestamp && (
        <span className="shrink-0 text-muted-foreground tabular-nums" title={entry.timestamp}>
          {shortTime ? formatLogClock(entry.time) : formatLogTime(entry.time)}
        </span>
      )}
      {showContainer && (
        <span className={cn("shrink-0 truncate font-medium", containerTone(entry.container).text)} style={{ width: `${tagWidth}ch` }} title={entry.container}>
          {entry.container}
        </span>
      )}
      <span className={cn("min-w-0 flex-1", wrap ? "whitespace-pre-wrap [overflow-wrap:anywhere]" : "whitespace-pre", tint(entry))}>
        {entry.ansi ? (
          <AnsiText text={entry.text} spans={entry.ansi} ranges={ranges} current={!!current} />
        ) : ranges.length > 0 ? (
          <MarkedText text={entry.text} ranges={ranges} current={!!current} />
        ) : (
          entry.text
        )}
      </span>
    </div>
  );
});
