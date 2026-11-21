import { useMemo } from "react";
import { Loader2 } from "lucide-react";
import type { ComposeDiskUsage, ComposeProject, Container } from "@bindings/model";
import { SortTh } from "@/components/tables/SortTh";
import { aggregateState } from "@/lib/composeGrouping";
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
  containers: (e) => e.p.total,
  cpu: (e) => e.totals?.cpu?.value,
  memory: (e) => e.totals?.memory?.value,
};

export function ProjectsTable({
  connectionId,
  projects,
  containers,
  live,
  disk,
}: {
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

  return (
    <table className="w-full table-fixed border-collapse text-[13px]">
      <colgroup>
        <col className="w-[18%]" />
        <col className="hidden w-[150px] @[1100px]:table-column" />
        <col className="w-[200px]" />
        <col className="w-[80px]" />
        <col className="w-[120px]" />
        <col className="w-[80px]" />
        <col className="w-[110px]" />
        <col className="hidden w-[120px] @[1300px]:table-column" />
        <col className="hidden @[1150px]:table-column" />
        <col className="w-[84px]" />
      </colgroup>
      <thead>
        <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
          <SortTh table="compose" id="name">Project</SortTh>
          <th className="hidden px-3 font-medium @[1100px]:table-cell">Source</th>
          <SortTh table="compose" id="state">State</SortTh>
          <th className="px-3 font-medium">Services</th>
          <SortTh table="compose" id="containers">Containers</SortTh>
          <SortTh table="compose" id="cpu" align="right">CPU</SortTh>
          <SortTh table="compose" id="memory" align="right">Memory</SortTh>
          <th className="hidden whitespace-nowrap px-3 text-right font-medium @[1300px]:table-cell">
            {disk.requested ? (
              <span className="inline-flex items-center gap-1">
                Disk {disk.loading && <Loader2 className="size-3 animate-spin" aria-label="Calculating" />}
              </span>
            ) : (
              <button type="button" onClick={disk.onCalculate} className="rounded px-1 py-0.5 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none" title="Ask the Engine for per-project disk usage (can be slow)">
                Disk · Calculate
              </button>
            )}
          </th>
          <th className="hidden px-3 font-medium @[1150px]:table-cell">Config</th>
          <th className="px-3 text-right font-medium">
            <span className="sr-only">Actions</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {entries.map(({ p, cs, totals }) => (
          <ProjectRow
            key={p.name}
            connectionId={connectionId}
            project={p}
            containers={cs}
            known={known}
            totals={totals}
            idle={live.idle.get(p.name)}
            diskLoaded={!!disk.data}
            diskLoading={disk.loading}
            disk={disk.data?.projects?.find((d) => d.project === p.name)}
          />
        ))}
      </tbody>
    </table>
  );
}
