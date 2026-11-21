import { useMemo, useRef } from "react";
import type { Container, ContainerStat } from "@bindings/model";
import { idleTracker, sameIdle, type IdleInfo } from "@/lib/idleTracker";
import { reuseTotals, totalsByProject, type ProjectTotals } from "@/lib/projectTotals";

export interface ProjectLive {
  totals: Map<string, ProjectTotals>;
  /** Projects that have stayed idle long enough (see lib/idleTracker); in-memory only. */
  idle: Map<string, IdleInfo>;
}

/**
 * Per-project CPU/memory totals aggregated from the stats that are already streamed, memoised on
 * the stats object (which changes at the stats cadence), plus the in-memory idle windows. The
 * maps are new on every tick, but a project's totals / idle info keep their previous object while
 * their values (and the idle chip text) did not change, so memoised rows can skip re-rendering.
 */
export function useProjectTotals(connectionId: string, containers: readonly Container[], stats: ReadonlyMap<string, ContainerStat>): ProjectLive {
  const prevRef = useRef<ProjectLive | null>(null);
  return useMemo(() => {
    const prev = prevRef.current;
    const totals = reuseTotals(prev?.totals, totalsByProject(containers, stats));
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
      if (info) {
        const old = prev?.idle.get(name);
        idle.set(name, old && sameIdle(old, info) ? old : info);
      }
    }
    const next = { totals, idle };
    prevRef.current = next;
    return next;
  }, [connectionId, containers, stats]);
}
