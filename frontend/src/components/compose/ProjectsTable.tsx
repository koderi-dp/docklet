import { useMemo } from "react";
import { Link } from "react-router-dom";
import type { ComposeProject, Container } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { SortTh } from "@/components/tables/SortTh";
import { aggregateState } from "@/lib/composeGrouping";
import { sortItems, type Accessors } from "@/lib/sorting";
import { useTableSort } from "@/lib/useTableSort";
import { projectContainers } from "@/hooks/useCompose";
import { ProjectActions } from "./ProjectActions";
import { ProjectHealthText, ProjectStateBadge } from "./ProjectStateBadge";

const STATE_RANK = { running: 0, partial: 1, transitioning: 2, stopped: 3 } as const;

interface Entry {
  p: ComposeProject;
  cs: Container[];
  /** False until the container list has loaded: the state is then unknown, not "stopped". */
  known: boolean;
}

const ACCESSORS: Accessors<Entry> = {
  name: (e) => e.p.name,
  state: (e) => (e.p.notCreated ? 4 : e.known ? STATE_RANK[aggregateState(e.cs)] : undefined),
  containers: (e) => e.p.total,
};

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

export function ProjectsTable({ connectionId, projects, containers }: { connectionId: string; projects: readonly ComposeProject[]; containers: readonly Container[] | null | undefined }) {
  const { sort } = useTableSort("compose");
  const known = !!containers;
  const entries = useMemo(
    () => sortItems(projects.map((p) => ({ p, cs: projectContainers(containers, p), known })), sort, ACCESSORS),
    [projects, containers, known, sort],
  );

  return (
    <table className="w-full table-fixed border-collapse text-[13px]">
      <colgroup>
        <col className="w-[24%]" />
        <col className="hidden w-[150px] @[1100px]:table-column" />
        <col className="w-[200px]" />
        <col className="w-[80px]" />
        <col className="w-[120px]" />
        <col className="hidden @[1000px]:table-column" />
        <col className="w-[84px]" />
      </colgroup>
      <thead>
        <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
          <SortTh table="compose" id="name">Project</SortTh>
          <th className="hidden px-3 font-medium @[1100px]:table-cell">Source</th>
          <SortTh table="compose" id="state">State</SortTh>
          <th className="px-3 font-medium">Services</th>
          <SortTh table="compose" id="containers">Containers</SortTh>
          <th className="hidden px-3 font-medium @[1000px]:table-cell">Config</th>
          <th className="px-3 text-right font-medium">
            <span className="sr-only">Actions</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {entries.map(({ p, cs }) => {
          const cfg = (p.configFiles ?? [])[0];
          return (
            <tr key={p.name} className="h-[46px] border-b border-border/60 transition-colors duration-100 last:border-b-0 hover:bg-muted/50">
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
              <td className="whitespace-nowrap px-3">
                <div className="flex items-center gap-2">
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
              <td className="hidden px-3 @[1000px]:table-cell">
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
        })}
      </tbody>
    </table>
  );
}
