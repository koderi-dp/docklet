import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useComposeProjects } from "@/hooks/useCompose";
import { useRenameComposeProject } from "@/hooks/useComposeClone";
import { renameBlocked } from "@/lib/cloneProject";
import { errorMessage } from "@/lib/errors";

interface Props {
  connectionId: string;
  project: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onRenamed?: (name: string) => void;
}

/** Renames a saved project that has no containers. Only the saved entry changes. */
export function RenameProjectDialog({ connectionId, project, open, onOpenChange, onRenamed }: Props) {
  const projects = useComposeProjects(open ? connectionId : null);
  const rename = useRenameComposeProject(connectionId);
  const [name, setName] = useState(project);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setName(project);
      setError(null);
    }
  }, [open, project]);

  const blocked = renameBlocked({ current: project, next: name, projects: projects.data ?? [] });
  const submit = () => {
    if (blocked || rename.isPending) return;
    const next = name.trim();
    setError(null);
    // Awaited, not mutate() callbacks: the old name disappears from the lists the moment the rename
    // lands, which can unmount this dialog before a callback would run.
    rename
      .mutateAsync({ name: project, newName: next })
      .then(() => {
        toast.success(`Renamed ${project} to ${next}`, { description: "Only the saved entry changed; the files are as they were." });
        onOpenChange(false);
        onRenamed?.(next);
      })
      .catch((e: unknown) => setError(errorMessage(e)));
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Rename project {project}</DialogTitle>
          <DialogDescription>
            The project has no containers, so only its saved name changes. The Compose files stay where they are, and containers, networks and volumes created later carry the new name.
          </DialogDescription>
        </DialogHeader>
        <form
          className="space-y-1.5"
          onSubmit={(e) => {
            e.preventDefault();
            submit();
          }}
        >
          <Label htmlFor="rename-project-name">New project name</Label>
          <Input
            id="rename-project-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            spellCheck={false}
            autoFocus
            className="h-8 font-mono text-xs"
          />
          {error && <p className="text-xs text-destructive">{error}</p>}
        </form>
        <DialogFooter className="items-center gap-2 sm:justify-between">
          <span className="text-xs text-muted-foreground">{blocked ?? ""}</span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button size="sm" disabled={!!blocked || rename.isPending} onClick={submit}>
              {rename.isPending ? "Renaming…" : "Rename"}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
