import { useState } from "react";
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
import { Checkbox } from "@/components/ui/checkbox";
import { useComposeResources } from "@/hooks/useCompose";
import type { Container } from "@bindings/model";

interface Props {
  connectionId: string;
  project: string;
  containers: readonly Container[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (opts: { volumes: boolean }) => void;
}

/** Compose Down removes containers and networks; volumes only via the separate, unchecked option. */
export function DownDialog({ connectionId, project, containers, open, onOpenChange, onConfirm }: Props) {
  const [volumes, setVolumes] = useState(false);
  const res = useComposeResources(connectionId, project, open);
  const networks = res.data?.networks ?? [];
  const vols = res.data?.volumes ?? [];
  return (
    <AlertDialog
      open={open}
      onOpenChange={(o) => {
        if (!o) setVolumes(false);
        onOpenChange(o);
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Run Compose Down for {project}?</AlertDialogTitle>
          <AlertDialogDescription>
            Down is not Stop: it stops and removes the project&apos;s containers and its networks. This cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className="max-h-64 space-y-3 overflow-auto text-sm">
          <div>
            <div className="mb-1 font-medium">Containers to remove ({containers.length})</div>
            <ul className="space-y-0.5 font-mono text-xs text-muted-foreground">
              {containers.map((c) => (
                <li key={c.id}>
                  {c.name} <span className="opacity-70">({c.state})</span>
                </li>
              ))}
              {containers.length === 0 && <li>None</li>}
            </ul>
          </div>
          <div>
            <div className="mb-1 font-medium">Networks to remove{res.data ? ` (${networks.length})` : ""}</div>
            {res.isLoading ? (
              <p className="text-xs text-muted-foreground">Loading…</p>
            ) : res.error ? (
              <p className="text-xs text-destructive">Could not list networks: {(res.error as Error).message}</p>
            ) : (
              <ul className="space-y-0.5 font-mono text-xs text-muted-foreground">
                {networks.map((n) => (
                  <li key={n.id}>{n.name}</li>
                ))}
                {networks.length === 0 && <li>None labelled for this project</li>}
              </ul>
            )}
          </div>
        </div>
        <label className="flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm">
          <Checkbox checked={volumes} onCheckedChange={(v) => setVolumes(v === true)} className="mt-0.5" />
          <span>
            <span className="font-medium">Also remove volumes (deletes data)</span>
            <span className="block text-xs text-muted-foreground">
              {vols.length > 0 ? `Project volumes: ${vols.map((v) => v.name).join(", ")}. ` : ""}
              Removed volumes and the data in them cannot be recovered.
            </span>
          </span>
        </label>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            onClick={() => {
              onConfirm({ volumes });
              setVolumes(false);
            }}
          >
            {volumes ? "Down and remove volumes" : "Down"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
