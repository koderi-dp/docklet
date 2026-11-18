const UNIT_SECONDS: Record<string, number> = { second: 1, minute: 60, hour: 3600, day: 86400, week: 604800, month: 2592000, year: 31536000 };

/**
 * Approximate seconds from the Engine's humanised uptime ("2 hours", "About an hour", "45 seconds",
 * "Less than a second"). Only used to order rows; undefined when it cannot be read.
 */
export function uptimeSeconds(label: string | undefined): number | undefined {
  if (!label) return undefined;
  const t = label.trim().toLowerCase();
  if (t.startsWith("less than a second")) return 0;
  const m = /^(?:about\s+)?(an?|\d+)\s+(second|minute|hour|day|week|month|year)s?\b/.exec(t);
  if (!m) return undefined;
  const n = m[1] === "a" || m[1] === "an" ? 1 : Number(m[1]);
  return n * UNIT_SECONDS[m[2]];
}
