// Merging of several container streams into one time-ordered buffer, and its size caps. Pure.
import type { LogEntry } from "./logEntry";

export const PER_CONTAINER_CAP = 5000;
export const OVERALL_CAP = 50000;

const before = (a: LogEntry, b: LogEntry) => a.sortKey < b.sortKey || (a.sortKey === b.sortKey && a.seq < b.seq);

/** Merges a batch into a time-ordered buffer. The common case (a batch newer than the buffer) is a plain append. */
export function mergeEntries(existing: readonly LogEntry[], incoming: readonly LogEntry[]): LogEntry[] {
  if (incoming.length === 0) return existing as LogEntry[];
  const batch = [...incoming].sort((a, b) => a.sortKey - b.sortKey || a.seq - b.seq);
  if (existing.length === 0) return batch;
  if (!before(batch[0], existing[existing.length - 1])) return existing.concat(batch);
  const out: LogEntry[] = new Array<LogEntry>(existing.length + batch.length);
  let i = 0;
  let j = 0;
  let k = 0;
  while (i < existing.length && j < batch.length) out[k++] = before(batch[j], existing[i]) ? batch[j++] : existing[i++];
  while (i < existing.length) out[k++] = existing[i++];
  while (j < batch.length) out[k++] = batch[j++];
  return out;
}

export interface Capped {
  lines: LogEntry[];
  /** How many lines were dropped (oldest first). */
  dropped: number;
}

/** Keeps at most `perContainer` lines of each container and `overall` lines in total, dropping the oldest. */
export function capEntries(lines: LogEntry[], perContainer = PER_CONTAINER_CAP, overall = OVERALL_CAP): Capped {
  const counts = new Map<string, number>();
  for (const l of lines) counts.set(l.container, (counts.get(l.container) ?? 0) + 1);
  const excess = new Map<string, number>();
  for (const [name, n] of counts) if (n > perContainer) excess.set(name, n - perContainer);

  let out = lines;
  let dropped = 0;
  if (excess.size > 0) {
    out = [];
    for (const l of lines) {
      const left = excess.get(l.container);
      if (left) {
        excess.set(l.container, left - 1);
        dropped++;
      } else out.push(l);
    }
  }
  if (out.length > overall) {
    dropped += out.length - overall;
    out = out.slice(out.length - overall);
  }
  return { lines: out, dropped };
}

/**
 * One container's lines in time order. The Engine delivers stdout and stderr as separate streams, so lines can
 * arrive out of order; they are placed by timestamp (arrival order breaks ties). Returns the input when it is
 * already in order, which is the normal case.
 */
export function sortByTime(lines: readonly LogEntry[]): readonly LogEntry[] {
  for (let i = 1; i < lines.length; i++) {
    if (before(lines[i], lines[i - 1])) return [...lines].sort((a, b) => a.sortKey - b.sortKey || a.seq - b.seq);
  }
  return lines;
}
