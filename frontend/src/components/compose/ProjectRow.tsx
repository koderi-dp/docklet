import { memo } from "react";
import { Link } from "react-router-dom";
import type { ComposeProject, Container } from "@bindings/model";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { IdleInfo } from "@/lib/idleTracker";
import type { ProjectTotals } from "@/lib/projectTotals";
import { configSubline } from "@/lib/projectRowModel";
import { ProjectActions } from "./ProjectActions";
import { ProjectResourcesCell } from "./ProjectResourcesCell";
import { ProjectStatusCell } from "./ProjectStatusCell";
import type { DiskState } from "./ProjectsTable";

interface ProjectRowProps {
  connectionId: string;
  project: ComposeProject;
  containers: Container[];
  /** False until the container list has loaded: the state is then unknown, not "stopped". */
  known: boolean;
  totals: ProjectTotals | undefined;
  idle: IdleInfo | undefined;
  disk: DiskState;
  diskEntry: NonNullable<NonNullable<DiskState["data"]>["projects"]>[number] | undefined;
}

export const ProjectRow = memo(function ProjectRow({ connectionId, project: p, containers: cs, known, totals, idle, disk, diskEntry }: ProjectRowProps) {
  const files = p.configFiles ?? [];
  return (
    <tr className="border-b border-border/60 transition-colors duration-100 last:border-b-0 hover:bg-muted/50">
      <td className="px-3 py-2 align-middle">
        <Tooltip>
          <TooltipTrigger asChild>
            <Link to={`/compose/${encodeURIComponent(p.name)}`} className="block truncate rounded font-semibold outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring/60">
              {p.name}
            </Link>
          </TooltipTrigger>
          <TooltipContent>{p.name}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger asChild>
            <div className="truncate text-xs text-muted-foreground">{configSubline(p)}</div>
          </TooltipTrigger>
          <TooltipContent className="max-w-lg break-all font-mono text-xs">{files.length > 0 ? files.join(", ") : p.source}</TooltipContent>
        </Tooltip>
      </td>
      <td className="px-3 py-2 align-middle">
        <ProjectStatusCell project={p} containers={cs} known={known} />
      </td>
      <td className="px-3 py-2 align-middle">
        <ProjectResourcesCell project={p} totals={totals} idle={idle} disk={disk} diskEntry={diskEntry} />
      </td>
      <td className="px-3 py-2 align-middle">
        <div className="flex justify-end">
          <ProjectActions connectionId={connectionId} project={p} containers={cs} />
        </div>
      </td>
    </tr>
  );
});
