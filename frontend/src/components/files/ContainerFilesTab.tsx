import { toast } from "sonner";
import { HardDriveDownload } from "lucide-react";
import { DialogService, FileService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { useOperations } from "@/hooks/useOperations";
import { useContainerFileBackend, useFileCaps } from "@/hooks/useFiles";
import { FileBrowser } from "./FileBrowser";
import { errorMessage } from "@/lib/errors";

/** Files tab of the container details page (Docker archive API; stopped containers work too). */
export function ContainerFilesTab({ connectionId, containerId, containerName, state }: { connectionId: string; containerId: string; containerName: string; state: string }) {
  const backend = useContainerFileBackend(connectionId, containerId);
  const caps = useFileCaps(connectionId, containerId, state);
  const { openOperations } = useOperations();

  const exportFs = async () => {
    try {
      const dest = await DialogService.PickSavePath("Export container filesystem", `${containerName}.tar`);
      if (!dest) return;
      const op = await FileService.ExportFilesystem(connectionId, containerId, dest);
      toast.info(`Export started: ${op.title}`, { description: "Progress is shown in Operations.", action: { label: "View", onClick: () => openOperations(op.id) } });
    } catch (e) {
      toast.error("Could not start the export", { description: errorMessage(e), duration: Infinity, closeButton: true });
    }
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      <FileBrowser
        backend={backend}
        className="flex-1"
        toolbarExtra={
          <Button size="sm" variant="outline" onClick={() => void exportFs()} title="Export the whole container filesystem as a tar archive" aria-label="Export filesystem">
            <HardDriveDownload className="size-3.5" /> Export filesystem…
          </Button>
        }
        note={state === "running" ? undefined : "container is not running"}
        writeDisabledReason={caps.data && !caps.data.canWrite ? caps.data.writeReason || "Not possible for this container." : null}
        deleteDisabledReason={caps.isLoading ? "Checking what this container supports…" : caps.data?.canDelete ? null : caps.data?.deleteReason || "Not possible for this container."}
      />
    </div>
  );
}
