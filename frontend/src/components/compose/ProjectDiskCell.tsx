import { Loader2 } from "lucide-react";
import type { ComposeProjectDisk } from "@bindings/model";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Dash } from "@/components/containers/ContainerCells";
import { formatBytes } from "@/components/containers/format";
import { projectData } from "@/lib/projectDisk";

/** "Project data" (containers + volumes) of a project on the Compose list; images are shown on the project page only. */
export function ProjectDiskCell({ loaded, loading, disk }: { loaded: boolean; loading: boolean; disk: ComposeProjectDisk | undefined }) {
  if (loading && !loaded) return <Loader2 className="ml-auto size-3.5 animate-spin text-muted-foreground" aria-label="Calculating" />;
  if (!loaded) return <Dash why="Not calculated yet. The Engine's disk accounting can be slow, so it runs on request." />;
  const d = projectData(disk);
  if (!d) return <Dash why="The Engine reported no size for this project's containers or volumes" />;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="tabular-nums">
          {formatBytes(d.bytes)}
          {d.note && <span className="text-muted-foreground">*</span>}
        </span>
      </TooltipTrigger>
      <TooltipContent>Project data: container writable layers + volumes{d.note ? `. ${d.note}` : ""}</TooltipContent>
    </Tooltip>
  );
}
