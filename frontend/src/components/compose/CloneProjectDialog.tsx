import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { TriangleAlert } from "lucide-react";
import { toast } from "sonner";
import type { ComposeClonePreview, ComposeLocation } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useComposeLocation, useComposeProjects, useComposeVerb } from "@/hooks/useCompose";
import { useCloneComposeProject, useClonePreview } from "@/hooks/useComposeClone";
import { cloneBlocked, cloneSizeText, LARGE_CLONE_BYTES } from "@/lib/cloneProject";
import { errorMessage } from "@/lib/errors";
import { PathRow, WindowsDriveNote } from "./pathFields";

export interface CloneBodyProps {
  connectionId: string;
  project: string;
  location: ComposeLocation | undefined;
  preview: ComposeClonePreview | undefined;
  previewError?: string;
  name: string;
  onName: (v: string) => void;
  dir: string;
  onDir: (v: string) => void;
  startAfter: boolean;
  onStartAfter: (v: boolean) => void;
  error?: string;
}

/** The fields and notices of the Clone dialog (no data fetching, so it can be rendered on its own). */
export function CloneBody(p: CloneBodyProps) {
  const pv = p.preview;
  const notes: string[] = [];
  if (pv && !pv.blocked) {
    if (pv.running) notes.push("The original is running. Files that change while they are copied, such as a database's data folder, may not be usable in the copy.");
    if (pv.sizeKnown && pv.sizeBytes > LARGE_CLONE_BYTES) notes.push("The folder is large; the copy will take a while. You can cancel it in Operations.");
    for (const f of pv.outside ?? []) notes.push(`${f} is outside the folder and is not copied: the copy keeps using this file.`);
    notes.push(...(pv.collisions ?? []));
  }
  return (
    <div className="space-y-4 text-sm">
      <div className="rounded-md border border-border bg-muted/40 p-3 text-xs">
        <div className="flex gap-2">
          <span className="shrink-0 text-muted-foreground">Copies the folder</span>
          <span className="min-w-0 break-all font-mono">{pv?.sourceDir || "…"}</span>
        </div>
        <div className="mt-1 text-muted-foreground">
          {p.previewError ? <span className="text-destructive">{p.previewError}</span> : pv ? (pv.blocked ? "" : cloneSizeText(pv)) : "Measuring the folder…"}
          {p.location ? ` · on ${p.location.executionLocation}` : ""}
        </div>
        {pv?.blocked && <p className="mt-1 text-destructive">{pv.blocked}</p>}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="clone-project-name">Name of the copy</Label>
        <Input id="clone-project-name" value={p.name} onChange={(e) => p.onName(e.target.value)} spellCheck={false} className="h-8 font-mono text-xs" />
      </div>

      <div className="space-y-1.5">
        <Label>Folder for the copy — new or empty</Label>
        <PathRow connectionId={p.connectionId} value={p.dir} onChange={p.onDir} placeholder="Folder the project is copied to" label="Folder for the copy" pick={{ kind: "folder", title: "Choose a folder for the copy" }} />
        <WindowsDriveNote location={p.location} paths={[p.dir]} />
      </div>

      {notes.length > 0 && (
        <div className="rounded-md border border-warning/40 bg-warning/10 p-3 text-xs">
          <div className="mb-1 flex items-center gap-1.5 font-medium">
            <TriangleAlert className="size-3.5 shrink-0 text-warning" aria-hidden /> Before you clone
          </div>
          <ul className="max-h-40 list-disc space-y-1 overflow-auto pl-5 text-muted-foreground">
            {notes.map((n) => (
              <li key={n} className="break-words">
                {n}
              </li>
            ))}
          </ul>
        </div>
      )}

      <p className="text-xs text-muted-foreground">
        Named volumes are not copied: the copy gets its own, empty volumes when it is first started. The original project is not changed.
      </p>

      <label className="flex items-center gap-2 text-xs">
        <Checkbox checked={p.startAfter} onCheckedChange={(v) => p.onStartAfter(v === true)} />
        Start the copy after cloning (<span className="font-mono">docker compose up -d</span>)
      </label>

      {p.error && <p className="whitespace-pre-wrap break-words text-xs text-destructive">{p.error}</p>}
    </div>
  );
}

interface Props {
  connectionId: string;
  project: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Copies a saved project's folder and saves the copy under another name. */
export function CloneProjectDialog({ connectionId, project, open, onOpenChange }: Props) {
  const loc = useComposeLocation(connectionId, open);
  const projects = useComposeProjects(open ? connectionId : null);
  const preview = useClonePreview(connectionId, project, open);
  const clone = useCloneComposeProject(connectionId);
  const verb = useComposeVerb(connectionId);
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [dir, setDir] = useState("");
  const [startAfter, setStartAfter] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [prefilled, setPrefilled] = useState(false);

  useEffect(() => {
    if (!open) {
      setPrefilled(false);
      setError(null);
      setStartAfter(false);
    }
  }, [open]);
  // The suggestions arrive with the preview; they fill the fields once and never overwrite typing.
  useEffect(() => {
    if (open && !prefilled && preview.data) {
      setName(preview.data.suggestedName);
      setDir(preview.data.suggestedDir);
      setPrefilled(true);
    }
  }, [open, prefilled, preview.data]);

  const blocked = cloneBlocked({ preview: preview.data, name, dir, projects: projects.data ?? [] });
  const submit = () => {
    if (blocked || clone.isPending) return;
    const newName = name.trim();
    setError(null);
    // Awaited, not mutate() callbacks: those are skipped when the dialog unmounts during the copy.
    clone
      .mutateAsync({ name: project, newName, dir: dir.trim() })
      .then(() => {
        toast.success(`Cloned ${project} to ${newName}`, { description: startAfter ? "Starting the copy." : "The copy is saved and not started." });
        onOpenChange(false);
        navigate(`/compose/${encodeURIComponent(newName)}`);
        if (startAfter) void verb.run("up", newName);
      })
      .catch((e: unknown) => setError(errorMessage(e)));
  };

  return (
    <Dialog open={open} onOpenChange={(o) => (clone.isPending ? undefined : onOpenChange(o))}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Clone project {project}</DialogTitle>
          <DialogDescription>Copies the whole project folder and saves the copy as a separate project with its own name.</DialogDescription>
        </DialogHeader>
        <CloneBody
          connectionId={connectionId}
          project={project}
          location={loc.data}
          preview={preview.data}
          previewError={preview.error ? errorMessage(preview.error) : undefined}
          name={name}
          onName={setName}
          dir={dir}
          onDir={setDir}
          startAfter={startAfter}
          onStartAfter={setStartAfter}
          error={error ?? undefined}
        />
        <DialogFooter className="items-center gap-2 sm:justify-between">
          <span className="min-w-0 text-xs text-muted-foreground">{clone.isPending ? "Copying… You can follow or cancel it in Operations." : (blocked ?? "")}</span>
          <div className="flex shrink-0 gap-2">
            <Button variant="outline" size="sm" disabled={clone.isPending} onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button size="sm" disabled={!!blocked || clone.isPending} onClick={submit}>
              {clone.isPending ? "Cloning…" : startAfter ? "Clone and start" : "Clone"}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
