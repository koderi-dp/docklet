import type { ComposeProject } from "@bindings/model";
import type { IdleInfo } from "@/lib/idleTracker";
import type { ProjectTotals } from "@/lib/projectTotals";
import { ProjectDiskCell } from "./ProjectDiskCell";
import { CpuTotal, IdleChip, MemoryTotal } from "./ProjectTotalCells";
import type { DiskState } from "./ProjectsTable";

type DiskEntry = NonNullable<NonNullable<DiskState["data"]>["projects"]>[number];

function DiskPart({ disk, entry }: { disk: DiskState; entry: DiskEntry | undefined }) {
  if (!disk.loading && !disk.data) {
    return (
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation();
          disk.onCalculate();
        }}
        className="rounded text-xs text-muted-foreground underline-offset-2 hover:underline focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none"
        title="Ask the Engine for per-project disk usage (can be slow)"
      >
        Calculate
      </button>
    );
  }
  return <ProjectDiskCell loaded={!!disk.data} loading={disk.loading} disk={entry} />;
}

/** Live CPU, memory and disk while the project runs; disk alone (and why there is no usage) while it does not. */
export function ProjectResourcesCell({
  project,
  totals,
  idle,
  disk,
  diskEntry,
}: {
  project: ComposeProject;
  totals: ProjectTotals | undefined;
  idle: IdleInfo | undefined;
  disk: DiskState;
  diskEntry: DiskEntry | undefined;
}) {
  if (project.notCreated) return <span className="text-xs text-muted-foreground">Not created yet</span>;
  const diskPart = (
    <span className="inline-flex items-center gap-1">
      <span className="text-muted-foreground">Disk</span> <DiskPart disk={disk} entry={diskEntry} />
    </span>
  );
  if (totals && totals.running > 0) {
    return (
      <div className="flex flex-col gap-0.5">
        <div className="flex flex-wrap items-center gap-x-1.5 gap-y-1">
          <span className="text-muted-foreground">CPU</span> <CpuTotal totals={totals} />
          <span className="text-muted-foreground">·</span>
          <span className="text-muted-foreground">Mem</span> <MemoryTotal totals={totals} />
          <IdleChip idle={idle} totals={totals} />
        </div>
        {diskPart}
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-0.5">
      {diskPart}
      <span className="text-xs text-muted-foreground">No live usage while stopped</span>
    </div>
  );
}
