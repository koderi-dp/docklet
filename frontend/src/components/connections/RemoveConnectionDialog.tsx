import { useState } from "react";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { useConnections } from "@/hooks/useActiveConnection";
import { useUI } from "@/stores/uiStore";
import { endpointSummary, typeLabel } from "./connectionMeta";
import { useConnectionActions } from "./useConnectionActions";

/** Confirmation for removing a saved connection. Mounted once in AppShell. */
export function RemoveConnectionDialogHost() {
  const id = useUI((s) => s.removeConnectionId);
  const setId = useUI((s) => s.setRemoveConnectionId);
  const { data: conns } = useConnections();
  const { remove } = useConnectionActions();
  const [busy, setBusy] = useState(false);
  const p = conns?.find((c) => c.id === id);
  return (
    <AlertDialog open={!!id && !!p} onOpenChange={(o) => !o && !busy && setId(null)}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Remove {p?.name}?</AlertDialogTitle>
          <AlertDialogDescription>
            This disconnects and removes the saved connection from Docklet. The Docker Engine and its containers, images
            and volumes are not touched.
          </AlertDialogDescription>
        </AlertDialogHeader>
        {p && (
          <div className="rounded-md border bg-muted/40 p-3 text-sm">
            <div className="font-medium">{p.name}</div>
            <div className="text-xs text-muted-foreground">
              {typeLabel(p.type)} · <span className="font-mono">{endpointSummary(p)}</span>
            </div>
          </div>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel>
          <Button
            variant="destructive"
            disabled={busy}
            onClick={async () => {
              if (!p) return;
              setBusy(true);
              const ok = await remove(p.id, p.name);
              setBusy(false);
              if (ok) setId(null);
            }}
          >
            Remove connection
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
