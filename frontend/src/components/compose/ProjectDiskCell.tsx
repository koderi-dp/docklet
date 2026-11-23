import { Loader2 } from "lucide-react";
import type { ComposeProjectDisk } from "@bindings/model";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { formatBytes } from "@/components/containers/format";
import { projectData } from "@/lib/projectDisk";

/** "Project data" (containers + volumes) of a project on the Compose list; images are shown on the project page only. */
export function ProjectDiskCell({ loaded, loading, disk }: { loaded: boolean; loading: boolean; disk: ComposeProjectDisk | undefined }) {
  if (loading && !loaded) return <Loader2 className="size-3.5 animate-spin text-muted-foreground" aria-label="Calculating" />;
  if (!loaded) return null;
  const d = projectData(disk);
  if (!d) return <span className="text-muted-foreground" title="The Engine reported no size for this project's containers or volumes">none</span>;
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
