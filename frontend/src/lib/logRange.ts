export type TimeRange = "5m" | "15m" | "1h" | "all";

export const TIME_RANGES: Array<{ id: TimeRange; label: string; ms: number | null }> = [
  { id: "5m", label: "Last 5 min", ms: 5 * 60_000 },
  { id: "15m", label: "Last 15 min", ms: 15 * 60_000 },
  { id: "1h", label: "Last hour", ms: 60 * 60_000 },
  { id: "all", label: "All", ms: null },
];

/** Epoch ms of an Engine RFC3339(Nano) timestamp; undefined when missing or unreadable. */
export function parseLogTime(ts: string | undefined): number | undefined {
  if (!ts) return undefined;
  const t = Date.parse(ts);
  return Number.isNaN(t) ? undefined : t;
}

/** Lines inside the range. Lines whose time cannot be read are kept (their age is unknown). */
export function inTimeRange(ts: string | undefined, range: TimeRange, now: number): boolean {
  const span = TIME_RANGES.find((r) => r.id === range)?.ms;
  if (span == null) return true;
  const t = parseLogTime(ts);
  return t === undefined || t >= now - span;
}
