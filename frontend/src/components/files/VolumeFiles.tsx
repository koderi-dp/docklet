import { useState } from "react";
import { toast } from "sonner";
import { DialogService, FileService } from "@bindings/services";
import type { Operation, Volume } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useOperations } from "@/hooks/useOperations";
import { useVolumeFileBackend } from "@/hooks/useFiles";
import { FileBrowser } from "./FileBrowser";
import { toastError } from "@/lib/errors";


/** Browse / Export / Import actions of the volume inspector. */
export function VolumeFileActions({ connectionId, volume }: { connectionId: string; volume: Volume }) {
  const [browse, setBrowse] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const { openOperations } = useOperations();
  const running = (volume.inUseBy ?? []).filter((u) => u.state === "running");

  const started = (op: Operation, what: string) =>
    toast.info(`${what} started: ${op.title}`, { description: "Progress is shown in Operations.", action: { label: "View", onClick: () => openOperations(op.id) } });

  const exportVolume = async () => {
    try {
      const dest = await DialogService.PickSavePath("Export volume as tar", `${volume.name}.tar`);
      if (!dest) return;
      started(await FileService.VolumeExport(connectionId, volume.name, dest), "Export");
    } catch (e) {
      toastError("Could not start the export", e);
    }
  };

  return (
    <>
      <div className="mt-4 flex flex-wrap gap-2">
        <Button size="sm" variant="outline" onClick={() => setBrowse(true)}>Browse</Button>
        <Button size="sm" variant="outline" onClick={() => void exportVolume()}>Export…</Button>
        <Button
          size="sm"
          variant="outline"
          disabled={running.length > 0}
          title={running.length > 0 ? `Stop ${running.map((u) => u.name).join(", ")} before importing` : undefined}
          onClick={() => setImportOpen(true)}
        >
          Import…
        </Button>
      </div>
      <p className="mt-2 text-xs text-muted-foreground">
        Browse and export use a temporary helper container created from an image already on this engine (nothing is pulled). It is removed afterwards.
        {running.length > 0 && " Import is disabled while running containers use this volume."}
      </p>
      <VolumeBrowseDialog open={browse} onOpenChange={setBrowse} connectionId={connectionId} volume={volume.name} />
      <ImportDialog open={importOpen} onOpenChange={setImportOpen} connectionId={connectionId} volume={volume.name} onStarted={(op) => started(op, "Import")} />
    </>
  );
}

function VolumeBrowseDialog({ open, onOpenChange, connectionId, volume }: { open: boolean; onOpenChange: (o: boolean) => void; connectionId: string; volume: string }) {
  const backend = useVolumeFileBackend(connectionId, volume);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[80vh] max-w-5xl flex-col sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle>Volume {volume}</DialogTitle>
          <DialogDescription>Read-only view through a temporary helper container. Select a file to preview or download it.</DialogDescription>
        </DialogHeader>
        {open && (
          <FileBrowser
            backend={backend}
            rootLabel={volume}
            className="min-h-0 flex-1"
            writeDisabledReason="Volumes are browsed read-only."
            deleteDisabledReason="Volumes are browsed read-only."
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function ImportDialog({
  open,
  onOpenChange,
  connectionId,
  volume,
  onStarted,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  connectionId: string;
  volume: string;
  onStarted: (op: Operation) => void;
}) {
  const [confirm, setConfirm] = useState(false);
  return (
    <AlertDialog
      open={open}
      onOpenChange={(o) => {
        onOpenChange(o);
        if (!o) setConfirm(false);
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Import into volume {volume}?</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div>
              <p>
                You will choose a .tar (or .tar.gz) file. Its contents are extracted into the root of <span className="font-mono">{volume}</span>. Files with
                the same path are overwritten and this cannot be undone. Archives with absolute or parent-directory paths are rejected.
              </p>
              <div className="mt-3 flex items-start gap-2">
                <Checkbox id="vol-import-confirm" checked={confirm} onCheckedChange={(v) => setConfirm(v === true)} />
                <Label htmlFor="vol-import-confirm" className="leading-snug">
                  I understand that existing files in this volume may be overwritten.
                </Label>
              </div>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            disabled={!confirm}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            onClick={() => {
              void (async () => {
                try {
                  const src = await DialogService.PickFile("Choose a tar archive", "Tar archives", "*.tar;*.tar.gz;*.tgz");
                  if (!src) return;
                  onStarted(await FileService.VolumeImport(connectionId, volume, src, true));
                } catch (e) {
                  toastError("Could not start the import", e);
                }
              })();
            }}
          >
            Choose file and import…
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
