import type { Network } from "@bindings/model";
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
import { networkAttached } from "./useNetworksPage";

interface Props {
  confirmRemove: Network[] | null;
  onCancelRemove: () => void;
  onRemove: (targets: Network[]) => void;
  pruneOpen: boolean;
  onPruneOpenChange: (o: boolean) => void;
  unused: Network[];
  onPrune: () => void;
}

/** Remove and prune confirmations: list every target and the impact. */
export function NetworkConfirmDialogs({ confirmRemove, onCancelRemove, onRemove, pruneOpen, onPruneOpenChange, unused, onPrune }: Props) {
  return (
    <>
      <AlertDialog open={!!confirmRemove} onOpenChange={(o) => !o && onCancelRemove()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove {confirmRemove?.length} network{confirmRemove?.length === 1 ? "" : "s"}?</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div>
                <p>These networks will be deleted. Networks with attached containers are refused by the Engine.</p>
                <ul className="mt-2 max-h-40 list-disc overflow-auto pl-5 font-mono text-xs">
                  {confirmRemove?.map((n) => (
                    <li key={n.id}>{n.name}{networkAttached(n) > 0 ? ` (attached: ${(n.attached ?? []).map((a) => a.name).join(", ")})` : ""}</li>
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
            <AlertDialogTitle>Prune unused networks?</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div>
                <p>Removes every network that no container uses. Predefined networks (bridge, host, none) are never removed.</p>
                {unused.length > 0 ? (
                  <>
                    <p className="mt-2 text-xs">Unused right now ({unused.length}); the Engine decides the final set when it runs:</p>
                    <ul className="mt-1 max-h-40 list-disc overflow-auto pl-5 font-mono text-xs">
                      {unused.map((n) => <li key={n.id}>{n.name}</li>)}
                    </ul>
                  </>
                ) : (
                  <p className="mt-2 text-xs">No unused networks in the current list.</p>
                )}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={() => onPrune()}>
              Prune unused networks
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
