import { useMemo } from "react";
import type { Container, ContainerStat } from "@bindings/model";
import { idleTracker, type IdleInfo } from "@/lib/idleTracker";
import { totalsByProject, type ProjectTotals } from "@/lib/projectTotals";

export interface ProjectLive {
  totals: Map<string, ProjectTotals>;
  /** Projects that have stayed idle long enough (see lib/idleTracker); in-memory only. */
  idle: Map<string, IdleInfo>;
}

/**
 * Per-project CPU/memory totals aggregated from the stats that are already streamed, memoised on
 * the stats object (which only changes at the stats cadence), plus the in-memory idle windows.
 */
export function useProjectTotals(connectionId: string, containers: readonly Container[], stats: ReadonlyMap<string, ContainerStat>): ProjectLive {
  return useMemo(() => {
    const totals = totalsByProject(containers, stats);
    const now = Date.now();
    const idle = new Map<string, IdleInfo>();
    for (const [name, t] of totals) {
      const key = `${connectionId}/${name}`;
      idleTracker.observe(key, now, {
        cpuPercent: t.cpu?.value ?? null,
        complete: !!t.cpu && t.cpu.reporting === t.running,
        membersKey: t.runningKey,
      });
      const info = idleTracker.idle(key, now);
      if (info) idle.set(name, info);
    }
    return { totals, idle };
  }, [connectionId, containers, stats]);
}
