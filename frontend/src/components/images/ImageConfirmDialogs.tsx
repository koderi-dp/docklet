import type { ImageRow } from "@bindings/model";
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
import { formatBytes } from "@/components/containers/format";
import { shortImageId } from "./ImagesTable";

interface Props {
  confirmRemove: ImageRow[] | null;
  onCancelRemove: () => void;
  force: boolean;
  onForceChange: (v: boolean) => void;
  onRemove: () => void;
  confirmPrune: boolean;
  onPruneOpenChange: (o: boolean) => void;
  danglingRows: ImageRow[];
  onPrune: () => void;
}

/** Remove-image and remove-dangling confirmation dialogs (list every target and the impact). */
export function ImageConfirmDialogs({ confirmRemove, onCancelRemove, force, onForceChange, onRemove, confirmPrune, onPruneOpenChange, danglingRows, onPrune }: Props) {
  return (
    <>
      <AlertDialog open={!!confirmRemove} onOpenChange={(o) => !o && onCancelRemove()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Remove {confirmRemove?.length} image reference{confirmRemove?.length === 1 ? "" : "s"}?
            </AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div>
                <p>
                  Removing a tag only untags the image; its layers are deleted when the last tag is removed and no container uses it.
                  Images used by containers cannot be removed unless forced.
                </p>
                <ul className="mt-2 max-h-40 list-disc overflow-auto pl-5 font-mono text-xs">
                  {confirmRemove?.map((r) => (
                    <li key={r.key}>
                      {r.dangling ? `${r.repository || "<none>"}:<none> (${shortImageId(r.id)})` : r.ref}
                      {r.inUse ? ` — used by ${r.containerCount} container${r.containerCount === 1 ? "" : "s"}` : ""}
                    </li>
                  ))}
                </ul>
                <label className="mt-3 flex items-center gap-2 text-[13px] text-foreground">
                  <input type="checkbox" checked={force} onChange={(e) => onForceChange(e.target.checked)} className="size-3.5 accent-primary" />
                  Force removal (can untag images that stopped containers still reference)
                </label>
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => onRemove()}>
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={confirmPrune} onOpenChange={onPruneOpenChange}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove dangling images?</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div>
                <p>
                  Deletes untagged images that no container uses. The Engine decides exactly which ones; currently {danglingRows.length} look
                  eligible ({formatBytes(danglingRows.reduce((n, r) => n + r.size, 0))}). This cannot be undone. Progress appears in Operations.
                </p>
                <ul className="mt-2 max-h-40 list-disc overflow-auto pl-5 font-mono text-xs">
                  {danglingRows.map((r) => (
                    <li key={r.key}>{shortImageId(r.id)}</li>
                  ))}
                </ul>
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => onPrune()}>
              Remove dangling
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
