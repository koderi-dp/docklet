import { memo } from "react";
import { Link } from "react-router-dom";
import type { ComposeDiskUsage, ComposeProject, Container } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { IdleInfo } from "@/lib/idleTracker";
import type { ProjectTotals } from "@/lib/projectTotals";
import { ProjectActions } from "./ProjectActions";
import { ProjectDiskCell } from "./ProjectDiskCell";
import { ProjectHealthText, ProjectStateBadge } from "./ProjectStateBadge";
import { CpuTotal, IdleChip, MemoryTotal } from "./ProjectTotalCells";

/** Keeps the file name visible: the directory part truncates, the last segment never does. */
function MiddleEllipsisPath({ value }: { value: string }) {
  const i = Math.max(value.lastIndexOf("/"), value.lastIndexOf("\\"));
  const dir = i >= 0 ? value.slice(0, i + 1) : "";
  const file = i >= 0 ? value.slice(i + 1) : value;
  return (
    <>
      <span className="min-w-0 truncate">{dir}</span>
      <span className="shrink-0">{file}</span>
    </>
  );
}

interface ProjectRowProps {
  connectionId: string;
  project: ComposeProject;
  containers: Container[];
  /** False until the container list has loaded: the state is then unknown, not "stopped". */
  known: boolean;
  totals: ProjectTotals | undefined;
  idle: IdleInfo | undefined;
  diskLoaded: boolean;
  diskLoading: boolean;
  disk: NonNullable<ComposeDiskUsage["projects"]>[number] | undefined;
}

export const ProjectRow = memo(function ProjectRow({ connectionId, project: p, containers: cs, known, totals, idle, diskLoaded, diskLoading, disk }: ProjectRowProps) {
  const cfg = (p.configFiles ?? [])[0];
  return (
    <tr className="h-[46px] border-b border-border/60 transition-colors duration-100 last:border-b-0 hover:bg-muted/50">
      <td className="px-3">
        <Tooltip>
          <TooltipTrigger asChild>
            <Link to={`/compose/${encodeURIComponent(p.name)}`} className="block truncate rounded font-semibold outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring/60">
              {p.name}
            </Link>
          </TooltipTrigger>
          <TooltipContent>{p.name}</TooltipContent>
        </Tooltip>
      </td>
      <td className="hidden whitespace-nowrap px-3 @[1100px]:table-cell">
        <Badge variant="outline" className="rounded-md font-normal text-muted-foreground">
          {p.source}
        </Badge>
      </td>
      <td className="px-3 py-1">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
          {p.notCreated ? (
            <ProjectStateBadge containers={[]} notCreated />
          ) : known ? (
            <ProjectStateBadge containers={cs} />
          ) : (
            <span className="text-muted-foreground">—</span>
          )}
          {known && !p.notCreated && <ProjectHealthText containers={cs} />}
        </div>
      </td>
      <td className="whitespace-nowrap px-3 tabular-nums">{(p.services ?? []).length}</td>
      <td className="whitespace-nowrap px-3 tabular-nums">
        {p.running} of {p.total} running
      </td>
      <td className="whitespace-nowrap px-3 text-right">
        <CpuTotal totals={p.notCreated ? undefined : totals} />
      </td>
      <td className="whitespace-nowrap px-3">
        <div className="flex items-center justify-end gap-2">
          <IdleChip idle={idle} totals={totals} />
          <MemoryTotal totals={p.notCreated ? undefined : totals} />
        </div>
      </td>
      <td className="hidden whitespace-nowrap px-3 text-right @[1300px]:table-cell">
        <div className="flex justify-end">
          <ProjectDiskCell loaded={diskLoaded} loading={diskLoading} disk={disk} />
        </div>
      </td>
      <td className="hidden overflow-hidden px-3 @[1150px]:table-cell">
        {cfg ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="flex min-w-0 font-mono text-xs text-muted-foreground">
                <MiddleEllipsisPath value={cfg} />
                {(p.configFiles ?? []).length > 1 && <span className="shrink-0 pl-1">+{(p.configFiles ?? []).length - 1}</span>}
              </span>
            </TooltipTrigger>
            <TooltipContent className="max-w-lg break-all font-mono text-xs">{(p.configFiles ?? []).join(", ")}</TooltipContent>
          </Tooltip>
        ) : (
          <span className="text-muted-foreground">—</span>
        )}
      </td>
      <td className="px-3">
        <div className="flex justify-end">
          <ProjectActions connectionId={connectionId} project={p} containers={cs} />
        </div>
      </td>
    </tr>
  );
});
