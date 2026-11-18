// The unit shown by every log view: one Engine log line, analysed once when it arrives.
import type { LogLine } from "@bindings/model";
import { stripAnsi } from "@/lib/ansi";
import { detectLevel } from "@/lib/logLevel";
import { parseLogTime } from "@/lib/logRange";
import type { FilterableLine } from "@/lib/logFilter/rules";

export interface LogEntry extends FilterableLine {
  /** Monotonic arrival number, stable across trimming (React key, tie-break when times are equal). */
  seq: number;
  stream: string;
  /** Engine timestamp as received ("" when the Engine sent none). */
  timestamp: string;
  /** Position in the merged stream: the Engine time, or the previous line's key when the time cannot be read. */
  sortKey: number;
}

/**
 * Builds an entry. `fallbackKey` orders lines whose timestamp is missing or unreadable
 * (they sort by arrival, right after the line that came before them).
 */
export function makeEntry(line: LogLine, container: string, seq: number, fallbackKey: number): LogEntry {
  const text = stripAnsi(line.text);
  const time = parseLogTime(line.timestamp);
  return {
    seq,
    container,
    stream: line.stream,
    timestamp: line.timestamp ?? "",
    text,
    level: detectLevel(text),
    time,
    sortKey: time ?? fallbackKey,
  };
}

const pad = (n: number, w = 2) => String(n).padStart(w, "0");

/** Local "MM-DD HH:mm:ss.SSS" of an epoch ms; "" when unknown. */
export function formatLogTime(ms: number | undefined): string {
  if (ms === undefined) return "";
  const d = new Date(ms);
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`;
}

/** Local "HH:mm:ss" of an epoch ms (the compact viewer); "" when unknown. */
export function formatLogClock(ms: number | undefined): string {
  if (ms === undefined) return "";
  const d = new Date(ms);
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

/** Plain-text form of lines for copy / download. Uses the Engine's own timestamps. */
export function formatEntries(entries: readonly LogEntry[], opts: { timestamps: boolean; container: boolean }): string {
  return entries
    .map((e) => {
      const parts: string[] = [];
      if (opts.timestamps && e.timestamp) parts.push(e.timestamp);
      if (opts.container) parts.push(`[${e.container}]`);
      parts.push(e.text);
      return parts.join(" ");
    })
    .join("\n");
}
