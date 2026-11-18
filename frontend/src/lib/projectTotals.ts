// Pure per-project resource totals from the stats that are already streamed (no extra streams).
import type { Container, ContainerStat } from "@bindings/model";

/** One summed metric. `reporting` of `running` containers contributed a real sample. */
export interface MetricTotal {
  value: number;
  reporting: number;
  running: number;
}

export interface ProjectTotals {
  project: string;
  running: number;
  /** Percent of ONE core, summed (same convention as the per-container cell); null = no sample at all. */
  cpu: MetricTotal | null;
  /** Memory in use, summed; limits differ per container so none is shown. null = no sample at all. */
  memory: MetricTotal | null;
  /** Running container ids (sorted): the idle window restarts when this set changes. */
  runningKey: string;
}

function usable(n: number | null | undefined): n is number {
  return n != null && Number.isFinite(n);
}

/** Totals of one project's containers. Only RUNNING containers count; unknown is never 0. */
export function aggregateProject(project: string, containers: readonly Container[], stats: ReadonlyMap<string, ContainerStat>): ProjectTotals {
  let running = 0;
  let cpu = 0;
  let cpuN = 0;
  let mem = 0;
  let memN = 0;
  const ids: string[] = [];
  for (const c of containers) {
    if (c.state !== "running") continue;
    running++;
    ids.push(c.id);
    const s = stats.get(c.id);
    if (usable(s?.cpuPercent)) {
      cpu += s.cpuPercent;
      cpuN++;
    }
    if (usable(s?.memoryBytes)) {
      mem += s.memoryBytes;
      memN++;
    }
  }
  return {
    project,
    running,
    cpu: cpuN > 0 ? { value: cpu, reporting: cpuN, running } : null,
    memory: memN > 0 ? { value: mem, reporting: memN, running } : null,
    runningKey: ids.sort().join(","),
  };
}

/** Totals for every Compose project (com.docker.compose.project label only), in one pass. */
export function totalsByProject(containers: readonly Container[], stats: ReadonlyMap<string, ContainerStat>): Map<string, ProjectTotals> {
  const byProject = new Map<string, Container[]>();
  for (const c of containers) {
    if (!c.composeProject) continue;
    const list = byProject.get(c.composeProject);
    if (list) list.push(c);
    else byProject.set(c.composeProject, [c]);
  }
  const out = new Map<string, ProjectTotals>();
  for (const [name, list] of byProject) out.set(name, aggregateProject(name, list, stats));
  return out;
}

/** "3 of 4 containers reporting" when data is partial, otherwise undefined. */
export function partialNote(m: MetricTotal | null): string | undefined {
  if (!m || m.reporting >= m.running) return undefined;
  return `${m.reporting} of ${m.running} containers reporting`;
}
