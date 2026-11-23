import { useMemo } from "react";
import type { ComposeDiskUsage, ComposeProject, Container } from "@bindings/model";
import { SortTh } from "@/components/tables/SortTh";
import { aggregateState } from "@/lib/composeGrouping";
import { filterProjects, isProjectRunning, type ProjectFilter } from "@/lib/projectRowModel";
import type { ProjectTotals } from "@/lib/projectTotals";
import { sortItems, type Accessors } from "@/lib/sorting";
import { useTableSort } from "@/lib/useTableSort";
import { projectContainers } from "@/hooks/useCompose";
import type { ProjectLive } from "@/hooks/useProjectTotals";
import { ProjectRow } from "./ProjectRow";

const STATE_RANK = { running: 0, partial: 1, transitioning: 2, stopped: 3 } as const;

interface Entry {
  p: ComposeProject;
  cs: Container[];
  /** False until the container list has loaded: the state is then unknown, not "stopped". */
  known: boolean;
  totals: ProjectTotals | undefined;
}

export interface DiskState {
  data: ComposeDiskUsage | undefined;
  loading: boolean;
  /** True once the user asked for it (the Engine call can be slow, so it never runs on its own). */
  requested: boolean;
  onCalculate: () => void;
}

const ACCESSORS: Accessors<Entry> = {
  name: (e) => e.p.name,
  state: (e) => (e.p.notCreated ? 4 : e.known ? STATE_RANK[aggregateState(e.cs)] : undefined),
};

export function ProjectsTable({
  connectionId,
  projects,
  containers,
  live,
  disk,
  filter = "all",
}: {
  /** Optional until the page passes it; "all" shows every project. */
  filter?: ProjectFilter;
  connectionId: string;
  projects: readonly ComposeProject[];
  containers: readonly Container[] | null | undefined;
  live: ProjectLive;
  disk: DiskState;
}) {
  const { sort } = useTableSort("compose");
  const known = !!containers;
  const byProject = useMemo(() => new Map(projects.map((p) => [p.name, projectContainers(containers, p)])), [projects, containers]);
  const entries = useMemo(
    () => sortItems(projects.map((p) => ({ p, cs: byProject.get(p.name) ?? [], known, totals: live.totals.get(p.name) })), sort, ACCESSORS),
    [projects, byProject, known, sort, live.totals],
  );
  const shown = useMemo(() => filterProjects(entries, filter, (e) => isProjectRunning(e.p, e.known ? e.cs : undefined)), [entries, filter]);

  return (
    <table className="w-full table-fixed border-collapse text-[13px]">
      <colgroup>
        <col className="w-[30%]" />
        <col className="w-[32%]" />
        <col />
        <col className="w-[150px]" />
      </colgroup>
      <thead>
        <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
          <SortTh table="compose" id="name">Project</SortTh>
          <SortTh table="compose" id="state">Status</SortTh>
          <th className="px-3 font-medium">Resources</th>
          <th className="px-3 text-right font-medium">
            <span className="sr-only">Actions</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {shown.length === 0 ? (
          <tr>
            <td colSpan={4} className="py-6 text-center text-muted-foreground">
              {filter === "stopped" ? "No stopped projects." : "No running projects."}
            </td>
          </tr>
        ) : (
          shown.map(({ p, cs, totals }) => (
            <ProjectRow
              key={p.name}
              connectionId={connectionId}
              project={p}
              containers={cs}
              known={known}
              totals={totals}
              idle={live.idle.get(p.name)}
              disk={disk}
              diskEntry={disk.data?.projects?.find((d) => d.project === p.name)}
            />
          ))
        )}
      </tbody>
    </table>
  );
}
