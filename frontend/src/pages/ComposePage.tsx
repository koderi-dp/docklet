import { useCallback, useEffect, useMemo, useState } from "react";
import { FilePlus, FileText, Layers } from "lucide-react";
import { toast } from "sonner";
import { Events } from "@wailsio/runtime";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { EmptyState } from "@/components/shared/EmptyState";
import { PageHeader } from "@/components/shared/PageHeader";
import { TableCard } from "@/components/shared/TableCard";
import { NewComposeDialog } from "@/components/compose/NewComposeDialog";
import { OpenComposeDialog } from "@/components/compose/OpenComposeDialog";
import { ProjectsTable } from "@/components/compose/ProjectsTable";
import { ProjectFilterChips } from "@/components/compose/ProjectFilterChips";
import { projectContainers, useComposeProjects } from "@/hooks/useCompose";
import { countProjectFilters, isProjectRunning } from "@/lib/projectRowModel";
import { useUI } from "@/stores/uiStore";
import { useComposeDisk } from "@/hooks/useComposeDisk";
import { useContainersQuery, useContainerStats } from "@/hooks/useContainers";
import { useProjectTotals } from "@/hooks/useProjectTotals";
import { composeFilesFromDrop } from "@/lib/composeDrop";
import { cn } from "@/lib/utils";

export default function ComposePage() {
  return <ConnectionGate>{({ connectionId }) => <ComposeView connectionId={connectionId} />}</ConnectionGate>;
}

/** Compose files dropped on the window (Go forwards them as "files-dropped"; see main.go). */
function useDroppedComposeFiles(onFiles: (files: string[]) => void) {
  useEffect(() => {
    const off = Events.On("files-dropped", (ev: { data?: unknown }) => {
      const files = composeFilesFromDrop(ev.data);
      if (files.length) onFiles(files);
      else toast.info("Not a Compose file", { description: "Drop a .yml or .yaml Compose file." });
    });
    return () => off();
  }, [onFiles]);
}

const NO_CONTAINERS: never[] = [];

function ComposeView({ connectionId }: { connectionId: string }) {
  const projects = useComposeProjects(connectionId);
  const containers = useContainersQuery(connectionId, true);
  const list = projects.data ?? [];
  // One stats watch for the running Compose containers; per-project totals are summed from it.
  const runningIds = useMemo(() => (containers.data ?? []).filter((c) => c.state === "running" && c.composeProject).map((c) => c.id), [containers.data]);
  const { stats } = useContainerStats(connectionId, runningIds, true);
  const live = useProjectTotals(connectionId, containers.data ?? NO_CONTAINERS, stats);
  const composeFilter = useUI((s) => s.composeFilter);
  const setComposeFilter = useUI((s) => s.setComposeFilter);
  const counts = useMemo(
    () => countProjectFilters(list, (p) => isProjectRunning(p, containers.data ? projectContainers(containers.data, p) : undefined)),
    [list, containers.data],
  );
  const [calcDisk, setCalcDisk] = useState(false);
  const disk = useComposeDisk(connectionId, calcDisk);
  const [openFile, setOpenFile] = useState(false);
  const [newFile, setNewFile] = useState(false);
  const [dropped, setDropped] = useState<string[] | undefined>();
  const onDrop = useCallback((files: string[]) => {
    setDropped(files);
    setOpenFile(true);
  }, []);
  useDroppedComposeFiles(onDrop);

  return (
    <div data-file-drop-target className={cn("h-full overflow-auto p-5", "[&.file-drop-target-active]:ring-2 [&.file-drop-target-active]:ring-inset [&.file-drop-target-active]:ring-primary/50")}>
      <PageHeader
        title="Compose"
        subtitle="Manage multi-service applications. Drop a Compose file here to add it."
        actions={
          <>
            <Button size="sm" variant="outline" onClick={() => setNewFile(true)}>
              <FilePlus className="size-3.5" /> New Compose File
            </Button>
            <Button size="sm" variant="outline" onClick={() => setOpenFile(true)}>
              <FileText className="size-3.5" /> Open Compose File
            </Button>
          </>
        }
      />
      <NewComposeDialog connectionId={connectionId} open={newFile} onOpenChange={setNewFile} projects={list} containers={containers.data ?? undefined} onOpenExisting={(path) => onDrop([path])} />
      <OpenComposeDialog connectionId={connectionId} open={openFile} onOpenChange={setOpenFile} initialFiles={dropped} />
      {projects.isLoading ? (
        <TableCard>
          <Skeleton className="h-10 w-full rounded-none" />
          {[0, 1].map((i) => <Skeleton key={i} className="h-[46px] w-full rounded-none border-b border-border/60" />)}
        </TableCard>
      ) : projects.error ? (
        <EmptyState title="Could not load Compose projects">
          <p>{(projects.error as Error).message}</p>
          <Button className="mt-3" size="sm" onClick={() => void projects.refetch()}>
            Retry
          </Button>
        </EmptyState>
      ) : list.length === 0 ? (
        <EmptyState icon={<Layers />} title="No Compose projects on this Engine">
          <p>Projects are discovered from the com.docker.compose.project label of existing containers, or saved with Open Compose File. None were found.</p>
          <Button className="mt-3" size="sm" onClick={() => setNewFile(true)}>
            <FilePlus className="size-3.5" /> New Compose File
          </Button>
        </EmptyState>
      ) : (
        <>
        <div className="mb-3 flex items-center">
          <ProjectFilterChips counts={counts} value={composeFilter} onChange={setComposeFilter} />
        </div>
        <TableCard>
          <ProjectsTable
            filter={composeFilter}
            connectionId={connectionId}
            projects={list}
            containers={containers.data}
            live={live}
            disk={{ data: disk.data, loading: disk.isFetching, requested: calcDisk || !!disk.data, onCalculate: () => (calcDisk ? void disk.refetch() : setCalcDisk(true)) }}
          />
        </TableCard>
        </>
      )}
    </div>
  );
}
