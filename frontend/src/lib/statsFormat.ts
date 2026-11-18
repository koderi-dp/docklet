// Pure helpers for live stats: absolute units next to percentages, and the Dashboard's top consumers.
import type { Container, ContainerStat } from "@bindings/model";
import { formatBytes } from "@/components/containers/format";

/** Memory as "used / limit (7.1%)"; the limit part is omitted when the Engine reports none. */
export function formatMemoryUsage(used: number, limit?: number | null): { text: string; percent?: number } {
  if (limit == null || limit <= 0) return { text: formatBytes(used) };
  const percent = (used / limit) * 100;
  return { text: `${formatBytes(used)} / ${formatBytes(limit)}`, percent };
}

/** "7.1%" for a percentage; one decimal below 10, none above. */
export function formatPercent(p: number): string {
  return `${p >= 10 ? p.toFixed(0) : p.toFixed(1)}%`;
}

/** Docker CPU percent is relative to ONE core (200% = two cores busy). */
export function cpuCores(percent: number): string {
  return `${(percent / 100).toFixed(2)} cores`;
}

export interface Consumer {
  container: Container;
  value: number;
}

/**
 * The n running containers using the most CPU or memory. Containers without a sample for that
 * metric are left out (never counted as zero).
 */
export function topConsumers(containers: readonly Container[], stats: ReadonlyMap<string, ContainerStat>, metric: "cpu" | "memory", n = 5): Consumer[] {
  const out: Consumer[] = [];
  for (const c of containers) {
    if (c.state !== "running") continue;
    const s = stats.get(c.id);
    const value = metric === "cpu" ? s?.cpuPercent : s?.memoryBytes;
    if (value != null && !Number.isNaN(value)) out.push({ container: c, value });
  }
  return out.sort((a, b) => b.value - a.value || a.container.name.localeCompare(b.container.name)).slice(0, n);
}
