import type { Container } from "@bindings/model";
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
import { isRunning } from "@/lib/composeGrouping";

interface Props {
  confirmKill: Container[] | null;
  onCancelKill: () => void;
  onKill: (targets: Container[]) => void;
  confirmRemove: Container[] | null;
  onCancelRemove: () => void;
  onRemove: (targets: Container[]) => void;
  forceNeeded: boolean;
}

/** Kill and delete confirmations: list every target and the impact. */
export function ContainerConfirmDialogs({ confirmKill, onCancelKill, onKill, confirmRemove, onCancelRemove, onRemove, forceNeeded }: Props) {
  return (
    <>
      <AlertDialog open={!!confirmKill} onOpenChange={(o) => !o && onCancelKill()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Kill {confirmKill?.length} container{confirmKill?.length === 1 ? "" : "s"}?</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div>
                <p>Sends SIGKILL to the main process immediately, without a graceful shutdown. Unsaved data may be lost.</p>
                <ul className="mt-2 max-h-40 list-disc overflow-auto pl-5 font-mono text-xs">
                  {confirmKill?.map((c) => (
                    <li key={c.id}>{c.name}</li>
                  ))}
                </ul>
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                const t = confirmKill ?? [];
                onCancelKill();
                onKill(t);
              }}
            >
              Kill
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={!!confirmRemove} onOpenChange={(o) => !o && onCancelRemove()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {confirmRemove?.length} container{confirmRemove?.length === 1 ? "" : "s"}?</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div>
                <p>This permanently removes the following containers. Volumes are not removed. This cannot be undone.</p>
                <ul className="mt-2 max-h-40 list-disc overflow-auto pl-5 font-mono text-xs">
                  {confirmRemove?.map((c) => (
                    <li key={c.id}>{c.name}{isRunning(c) ? " (running)" : ""}</li>
                  ))}
                </ul>
                {forceNeeded && <p className="mt-2 text-destructive">Running containers will be force-killed before removal.</p>}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                const t = confirmRemove ?? [];
                onCancelRemove();
                onRemove(t);
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
