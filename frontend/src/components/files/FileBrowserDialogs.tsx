import { useEffect, useState } from "react";
import type { FileEntry } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { errorMessage } from "@/lib/errors";
import { baseName } from "./paths";

export function DeleteFileDialog({ entry, onCancel, onConfirm }: { entry: FileEntry | null; onCancel: () => void; onConfirm: (e: FileEntry) => void }) {
  return (
    <AlertDialog open={entry !== null} onOpenChange={(o) => !o && onCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {entry?.isSymlink ? "link" : entry?.isDir ? "directory" : "file"}?</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div>
              <p className="break-all font-mono text-foreground">{entry?.path}</p>
              <p className="mt-2">
                This runs <span className="font-mono">rm -rf</span> inside the running container
                {entry?.isDir && !entry.isSymlink ? " and removes the directory with everything in it" : ""}. It cannot be undone.
              </p>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            onClick={() => {
              onCancel();
              if (entry) onConfirm(entry);
            }}
          >
            Delete {baseName(entry?.path ?? "")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export function MkdirDialog({ open, onOpenChange, parent, onCreate }: { open: boolean; onOpenChange: (o: boolean) => void; parent: string; onCreate: (name: string) => Promise<void> }) {
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open) {
      setName("");
      setError(null);
    }
  }, [open]);
  const invalid = name.trim() === "" || name === "." || name === ".." || name.includes("/") ? "Enter a single folder name (no slashes)." : null;
  const submit = async () => {
    if (invalid) return setError(invalid);
    setBusy(true);
    try {
      await onCreate(name.trim());
      onOpenChange(false);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New folder</DialogTitle>
          <DialogDescription>Creates a folder in <span className="font-mono">{parent}</span>.</DialogDescription>
        </DialogHeader>
        <form onSubmit={(e) => { e.preventDefault(); void submit(); }}>
          <Input autoFocus value={name} onChange={(e) => { setName(e.target.value); setError(null); }} aria-label="Folder name" placeholder="folder name" />
          {error && <p className="mt-2 text-xs text-destructive">{error}</p>}
          <DialogFooter className="mt-4">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={busy}>Create</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
