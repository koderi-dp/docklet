import { useState } from "react";
import { FolderOpen, Info, Plus, X } from "lucide-react";
import { toast } from "sonner";
import { DialogService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAddComposeProject, useComposeLocation } from "@/hooks/useCompose";
import { errorMessage } from "@/lib/errors";

interface Props {
  connectionId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAdded?: (name: string) => void;
}


/** One path row: a text field (typed or picked) and a native picker button. */
function FileRow({
  value,
  onChange,
  onRemove,
  placeholder,
  title,
  pattern,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  onRemove?: () => void;
  placeholder: string;
  title: string;
  pattern: string;
  label: string;
}) {
  const pick = async () => {
    try {
      const p = await DialogService.PickFile(title, label, pattern);
      if (p) onChange(p);
    } catch (e) {
      toast.error("Could not open the file dialog", { description: errorMessage(e) });
    }
  };
  return (
    <div className="flex items-center gap-1.5">
      <Input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} spellCheck={false} className="h-8 font-mono text-xs" aria-label={label} />
      <Button type="button" size="sm" variant="outline" onClick={() => void pick()}>
        <FolderOpen className="size-3.5" /> Browse
      </Button>
      {onRemove && (
        <Button type="button" size="icon" variant="ghost" className="size-8" onClick={onRemove} aria-label="Remove this file" title="Remove this file">
          <X className="size-3.5" />
        </Button>
      )}
    </div>
  );
}

/** Registers a managed Compose project: validates the files where the CLI runs, then saves the record. */
export function OpenComposeDialog({ connectionId, open, onOpenChange, onAdded }: Props) {
  const loc = useComposeLocation(connectionId, open);
  const add = useAddComposeProject(connectionId);
  const [files, setFiles] = useState<string[]>([""]);
  const [envFile, setEnvFile] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);

  const l = loc.data;
  const usable = files.filter((f) => f.trim() !== "");
  const blocked = !l ? "Checking where Compose runs…" : !l.available ? l.reason || "Compose is not available for this connection" : !l.managedAvailable ? "Saving projects is not available: the application database is not open" : usable.length === 0 ? "Choose at least one Compose file" : undefined;

  const reset = () => {
    setFiles([""]);
    setEnvFile("");
    setName("");
    setError(null);
  };

  const submit = async () => {
    setError(null);
    try {
      const p = await add.mutateAsync({ configFiles: usable.map((f) => f.trim()), envFiles: envFile.trim() ? [envFile.trim()] : [], name: name.trim() });
      toast.success(`Project ${p.name} saved`, { description: "Nothing was started. Use Up to create its containers." });
      reset();
      onOpenChange(false);
      onAdded?.(p.name);
    } catch (e) {
      setError(errorMessage(e));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) reset();
        onOpenChange(o);
      }}
    >
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Open Compose File</DialogTitle>
          <DialogDescription>
            Save a Compose project from files, even when it is not running yet. The files are validated with <span className="font-mono">docker compose config</span>; nothing is started or copied.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 text-sm">
          <div className="rounded-md border border-border bg-muted/40 p-3 text-xs">
            <div>
              <span className="text-muted-foreground">Files are read, and Compose runs, on: </span>
              <span className="font-medium">{l ? l.executionLocation : "—"}</span>
            </div>
            {l?.pathNote && (
              <p className="mt-1 flex gap-1.5 text-muted-foreground">
                <Info className="mt-0.5 size-3.5 shrink-0" aria-hidden /> {l.pathNote}
              </p>
            )}
            {l?.warning && <p className="mt-1 text-warning">{l.warning}</p>}
            {l && !l.available && <p className="mt-1 text-destructive">{l.reason}</p>}
            {loc.error && <p className="mt-1 text-destructive">{(loc.error as Error).message}</p>}
          </div>

          <div className="space-y-1.5">
            <Label>Compose file(s) — later files override earlier ones</Label>
            {files.map((f, i) => (
              <FileRow
                key={i}
                value={f}
                onChange={(v) => setFiles((fs) => fs.map((x, j) => (j === i ? v : x)))}
                onRemove={files.length > 1 ? () => setFiles((fs) => fs.filter((_, j) => j !== i)) : undefined}
                placeholder={l?.kind === "wsl" ? "/home/user/app/compose.yaml" : "Absolute path to compose.yaml"}
                title="Choose a Compose file"
                pattern="*.yml;*.yaml"
                label="Compose file"
              />
            ))}
            <Button type="button" size="sm" variant="ghost" onClick={() => setFiles((fs) => [...fs, ""])}>
              <Plus className="size-3.5" /> Add another file
            </Button>
          </div>

          <div className="space-y-1.5">
            <Label>Environment file (optional)</Label>
            <FileRow value={envFile} onChange={setEnvFile} placeholder="Path to a .env file" title="Choose an environment file" pattern="" label="Environment file" />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="compose-name">Project name (optional)</Label>
            <Input id="compose-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Defaults to the name in the file or the directory name" spellCheck={false} className="h-8" />
          </div>

          {error && (
            <div role="alert" className="max-h-40 overflow-auto whitespace-pre-wrap rounded-md border border-destructive/40 bg-destructive/5 p-3 text-xs text-destructive">
              {error}
            </div>
          )}
        </div>

        <DialogFooter className="items-center gap-2 sm:justify-between">
          <span className="text-xs text-muted-foreground">{blocked ?? ""}</span>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button size="sm" disabled={!!blocked || add.isPending} onClick={() => void submit()}>
              {add.isPending ? "Validating…" : "Validate and save"}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
