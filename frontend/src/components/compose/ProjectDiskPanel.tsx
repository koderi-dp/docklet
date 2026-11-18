import { Loader2, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Dash } from "@/components/containers/ContainerCells";
import { formatBytes } from "@/components/containers/format";
import { useComposeDisk } from "@/hooks/useComposeDisk";
import { projectData } from "@/lib/projectDisk";

function Row({ label, hint, value, note, strong }: { label: string; hint?: string; value: number | null | undefined; note?: string; strong?: boolean }) {
  return (
    <div className="flex items-baseline gap-3 border-b border-border/60 px-3 py-2 last:border-b-0">
      <div className="min-w-0 flex-1">
        <span className={strong ? "font-semibold" : "font-medium"}>{label}</span>
        {hint && <span className="ml-2 text-xs text-muted-foreground">{hint}</span>}
      </div>
      <div className="text-right tabular-nums">
        {value == null ? (
          <Dash why="The Engine reported no size" />
        ) : note ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className={strong ? "font-semibold" : undefined}>
                {formatBytes(value)}
                <span className="text-muted-foreground">*</span>
              </span>
            </TooltipTrigger>
            <TooltipContent>{note}</TooltipContent>
          </Tooltip>
        ) : (
          <span className={strong ? "font-semibold" : undefined}>{formatBytes(value)}</span>
        )}
      </div>
    </div>
  );
}

/** Disk footprint of one project from the Engine's disk accounting; loaded when this panel mounts (the call can be slow). */
export function ProjectDiskPanel({ connectionId, project }: { connectionId: string; project: string }) {
  const q = useComposeDisk(connectionId, true);
  const d = q.data?.projects?.find((p) => p.project === project);
  const data = projectData(d);
  const refresh = (
    <Button variant="ghost" size="sm" className="h-7 gap-1 px-2 text-xs" disabled={q.isFetching} onClick={() => void q.refetch()}>
      {q.isFetching ? <Loader2 className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />} Recalculate
    </Button>
  );
  return (
    <section>
      <div className="mb-2 flex items-center justify-between">
        <h3 className="text-sm font-semibold">Disk</h3>
        {q.data && refresh}
      </div>
      {q.isLoading ? (
        <p className="flex items-center gap-2 text-muted-foreground">
          <Loader2 className="size-4 animate-spin" /> Asking the Engine for disk usage. This can take a while.
        </p>
      ) : q.error ? (
        <p className="text-destructive">
          Could not read disk usage: {(q.error as Error).message}{" "}
          <button className="underline" onClick={() => void q.refetch()}>
            Retry
          </button>
        </p>
      ) : !d ? (
        <p className="text-muted-foreground">The Engine reported no disk usage for this project (no containers or volumes carry its label).</p>
      ) : (
        <div className="rounded-[10px] border border-border bg-card text-[13px]">
          <Row label="Containers" hint={`writable layers, ${d.containerCount} container${d.containerCount === 1 ? "" : "s"}`} value={d.containersBytes} note={d.containersKnown < d.containerCount ? `${d.containersKnown} of ${d.containerCount} containers report a size` : undefined} />
          <Row label="Volumes" hint={`${d.volumeCount} volume${d.volumeCount === 1 ? "" : "s"}`} value={d.volumesBytes} note={d.volumesKnown < d.volumeCount ? `${d.volumesKnown} of ${d.volumeCount} volumes report a size` : undefined} />
          <Row label="Project data" hint="containers + volumes" value={data?.bytes} note={data?.note} strong />
          <Row
            label="Images"
            hint={`${d.imageCount} unique image${d.imageCount === 1 ? "" : "s"}, may be shared with other projects`}
            value={d.imagesBytes}
            note={d.imagesKnown < d.imageCount ? `${d.imagesKnown} of ${d.imageCount} images report a size` : undefined}
            />
        </div>
      )}
    </section>
  );
}
