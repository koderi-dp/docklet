import type { ComposeProject, Container } from "@bindings/model";
import type { IdleInfo } from "@/lib/idleTracker";
import type { ProjectTotals } from "@/lib/projectTotals";
import { CpuTotal, IdleChip, MemoryTotal, StopIdleProject } from "./ProjectTotalCells";

/** Project page summary: live CPU / memory totals, the idle hint and its Stop shortcut. Hidden when nothing runs. */
export function ProjectTotalsStrip({
  connectionId,
  project,
  containers,
  totals,
  idle,
}: {
  connectionId: string;
  project: ComposeProject;
  containers: readonly Container[];
  totals: ProjectTotals | undefined;
  idle: IdleInfo | undefined;
}) {
  if (project.notCreated || !totals || totals.running === 0) return null;
  return (
    <div className="mb-3 flex min-h-[46px] flex-wrap items-center gap-x-5 gap-y-2 rounded-[10px] border border-border bg-card px-3 py-2 text-[13px]">
      <span className="flex items-baseline gap-2">
        <span className="text-xs text-muted-foreground">CPU</span>
        <CpuTotal totals={totals} />
      </span>
      <span className="flex items-baseline gap-2">
        <span className="text-xs text-muted-foreground">Memory</span>
        <MemoryTotal totals={totals} />
      </span>
      <span className="text-xs text-muted-foreground">
        {totals.running} running container{totals.running === 1 ? "" : "s"}
      </span>
      {idle && (
        <span className="flex items-center gap-2">
          <IdleChip idle={idle} totals={totals} />
          <StopIdleProject connectionId={connectionId} project={project} containers={containers} />
        </span>
      )}
    </div>
  );
}
