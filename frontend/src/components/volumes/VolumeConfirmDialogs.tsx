import type { Volume } from "@bindings/model";
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
import { volumeUsers } from "./useVolumesPage";

interface Props {
  confirmRemove: Volume[] | null;
  onCancelRemove: () => void;
  onRemove: (targets: Volume[]) => void;
  pruneOpen: boolean;
  onPruneOpenChange: (o: boolean) => void;
  unusedVolumes: Volume[];
  understood: boolean;
  onUnderstood: (v: boolean) => void;
  onPrune: () => void;
}

/** Remove and prune confirmations: list every target and state that deleted data cannot be recovered. */
export function VolumeConfirmDialogs({ confirmRemove, onCancelRemove, onRemove, pruneOpen, onPruneOpenChange, unusedVolumes, understood, onUnderstood, onPrune }: Props) {
  return (
    <>
      <AlertDialog open={!!confirmRemove} onOpenChange={(o) => !o && onCancelRemove()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove {confirmRemove?.length} volume{confirmRemove?.length === 1 ? "" : "s"}?</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div>
                <p>The data stored in these volumes is deleted permanently and cannot be recovered. Volumes used by a container are refused by the Engine.</p>
                <ul className="mt-2 max-h-40 list-disc overflow-auto pl-5 font-mono text-xs">
                  {confirmRemove?.map((v) => (
                    <li key={v.name}>{v.name}{volumeUsers(v) > 0 ? ` (in use by ${(v.inUseBy ?? []).map((u) => u.name).join(", ")})` : ""}</li>
                  ))}
                </ul>
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive! text-destructive-foreground! hover:bg-destructive/90!"
              onClick={() => {
                const t = confirmRemove ?? [];
                onCancelRemove();
                onRemove(t);
              }}
            >
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={pruneOpen} onOpenChange={onPruneOpenChange}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Prune unused volumes?</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div>
                <p>
                  Removes every volume that no container (running or stopped) references, including named volumes. Persistent data in them is lost permanently.
                </p>
                {unusedVolumes.length > 0 ? (
                  <>
                    <p className="mt-2 text-xs">Unused right now ({unusedVolumes.length}); the Engine decides the final set when it runs:</p>
                    <ul className="mt-1 max-h-40 list-disc overflow-auto pl-5 font-mono text-xs">
                      {unusedVolumes.map((v) => <li key={v.name} className="break-all">{v.name}</li>)}
                    </ul>
                  </>
                ) : (
                  <p className="mt-2 text-xs">No unused volumes in the current list.</p>
                )}
                <label className="mt-3 flex items-center gap-2 text-[13px] text-foreground">
                  <Checkbox checked={understood} onCheckedChange={(c) => onUnderstood(c === true)} />
                  I understand that this deletes data permanently
                </label>
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction disabled={!understood} className="bg-destructive! text-destructive-foreground! hover:bg-destructive/90!" onClick={() => onPrune()}>
              Prune unused volumes
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
