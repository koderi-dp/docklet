import { Loader2 } from "lucide-react";
import type { ImageRow } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { formatDateTime } from "@/components/containers/format";
import { openUpdateDialog, useUpdateStatus } from "./updateStore";

/** Update state of one image row; renders nothing until a check was made. */
export function UpdateChip({ row }: { row: Pick<ImageRow, "connectionId" | "ref" | "dangling"> }) {
  const { status, checking } = useUpdateStatus(row.connectionId, row.ref);
  if (row.dangling) return null;
  if (checking) {
    return (
      <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
        <Loader2 className="size-3 animate-spin" /> Checking…
      </span>
    );
  }
  if (!status) return null;
  if (status.error) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="cursor-default text-xs text-muted-foreground">Can’t check</span>
        </TooltipTrigger>
        <TooltipContent className="max-w-80">{status.error}</TooltipContent>
      </Tooltip>
    );
  }
  if (status.updateAvailable) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            className="rounded-full focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none"
            onClick={(e) => {
              e.stopPropagation();
              openUpdateDialog(row);
            }}
          >
            <Badge variant="secondary" className="bg-info/15 font-normal text-info">
              Update available
            </Badge>
          </button>
        </TooltipTrigger>
        <TooltipContent>The registry has a newer image for this tag. Click to update.</TooltipContent>
      </Tooltip>
    );
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="cursor-default text-xs text-muted-foreground">Up to date</span>
      </TooltipTrigger>
      <TooltipContent>Checked {formatDateTime(new Date(status.checkedAt))}</TooltipContent>
    </Tooltip>
  );
}
