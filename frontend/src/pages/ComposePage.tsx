import { useState } from "react";
import { Link } from "react-router-dom";
import { FileText, Layers } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { EmptyState } from "@/components/shared/EmptyState";
import { PageHeader } from "@/components/shared/PageHeader";
import { TableCard } from "@/components/shared/TableCard";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { OpenComposeDialog } from "@/components/compose/OpenComposeDialog";
import { ProjectActions } from "@/components/compose/ProjectActions";
import { ProjectHealthText, ProjectStateBadge } from "@/components/compose/ProjectStateBadge";
import { useComposeProjects, projectContainers } from "@/hooks/useCompose";
import { useContainersQuery } from "@/hooks/useContainers";

export default function ComposePage() {
  return <ConnectionGate>{({ connectionId }) => <ComposeView connectionId={connectionId} />}</ConnectionGate>;
}

function ComposeView({ connectionId }: { connectionId: string }) {
  const projects = useComposeProjects(connectionId);
  const containers = useContainersQuery(connectionId, true);
  const list = projects.data ?? [];
  const [openFile, setOpenFile] = useState(false);

  return (
    <div className="h-full overflow-auto p-5">
      <PageHeader
        title="Compose"
        subtitle="Manage multi-service applications"
        actions={
          <Button size="sm" variant="outline" onClick={() => setOpenFile(true)}>
            <FileText className="size-3.5" /> Open Compose File
          </Button>
        }
      />
      <OpenComposeDialog connectionId={connectionId} open={openFile} onOpenChange={setOpenFile} />
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
        </EmptyState>
      ) : (
        <TableCard>
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
                <th className="h-10 px-3 font-medium">Project</th>
                <th className="hidden px-3 font-medium @[1100px]:table-cell">Source</th>
                <th className="px-3 font-medium">State</th>
                <th className="px-3 font-medium">Services</th>
                <th className="px-3 font-medium">Containers</th>
                <th className="hidden px-3 font-medium @[1000px]:table-cell">Config</th>
                <th className="px-3 text-right font-medium">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {list.map((p) => {
                const cs = projectContainers(containers.data, p);
                const known = !!containers.data;
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
        </TableCard>
      )}
    </div>
  );
}

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
