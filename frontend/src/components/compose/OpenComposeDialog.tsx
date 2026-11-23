import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { CheckCircle2, Plus } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { useAddComposeProject, useComposeLocation, useComposeVerb } from "@/hooks/useCompose";
import { errorMessage } from "@/lib/errors";
import { ComposeLocationBox, PathRow } from "./pathFields";

interface Props {
  connectionId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAdded?: (name: string) => void;
  /** Compose file paths to prefill (e.g. dropped onto the Compose page). */
  initialFiles?: string[];
}


/** Registers a managed Compose project: validates the files where the CLI runs, then saves the record. */
export function OpenComposeDialog({ connectionId, open, onOpenChange, onAdded, initialFiles }: Props) {
  const loc = useComposeLocation(connectionId, open);
  const add = useAddComposeProject(connectionId);
  const [files, setFiles] = useState<string[]>([""]);
  const [envFile, setEnvFile] = useState("");
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [startAfter, setStartAfter] = useState(true);
  const [validated, setValidated] = useState<string | null>(null);
  const verb = useComposeVerb(connectionId);
  const navigate = useNavigate();

  useEffect(() => {
    if (open && initialFiles?.length) setFiles(initialFiles);
  }, [open, initialFiles]);

  const l = loc.data;
  const usable = files.filter((f) => f.trim() !== "");
  const blocked = !l ? "Checking where Compose runs…" : !l.available ? l.reason || "Compose is not available for this connection" : !l.managedAvailable ? "Saving projects is not available: the application database is not open" : usable.length === 0 ? "Choose at least one Compose file" : undefined;

  const reset = () => {
    setFiles([""]);
    setEnvFile("");
    setName("");
    setError(null);
    setValidated(null);
  };

  const submit = async () => {
    setError(null);
    try {
      const p = await add.mutateAsync({ configFiles: usable.map((f) => f.trim()), envFiles: envFile.trim() ? [envFile.trim()] : [], name: name.trim() });
      setValidated(p.name); // the validation result stays visible while the project starts
      if (startAfter) {
        await verb.run("up", p.name);
        navigate(`/compose/${encodeURIComponent(p.name)}`);
      } else {
        toast.success(`Project ${p.name} saved`, { description: "Nothing was started. Use Up to create its containers." });
      }
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
          <ComposeLocationBox location={l} error={loc.error} />

          <div className="space-y-1.5">
            <Label>Compose file(s) — later files override earlier ones</Label>
            {files.map((f, i) => (
              <PathRow
                key={i}
                value={f}
                onChange={(v) => setFiles((fs) => fs.map((x, j) => (j === i ? v : x)))}
                onRemove={files.length > 1 ? () => setFiles((fs) => fs.filter((_, j) => j !== i)) : undefined}
                placeholder={l?.kind === "wsl" ? "/home/user/app/compose.yaml" : "Absolute path to compose.yaml"}
                pick={{ kind: "file", title: "Choose a Compose file", pattern: "*.yml;*.yaml" }}
                label="Compose file"
              />
            ))}
            <Button type="button" size="sm" variant="ghost" onClick={() => setFiles((fs) => [...fs, ""])}>
              <Plus className="size-3.5" /> Add another file
            </Button>
          </div>

          <div className="space-y-1.5">
            <Label>Environment file (optional)</Label>
            <PathRow value={envFile} onChange={setEnvFile} placeholder="Path to a .env file" pick={{ kind: "file", title: "Choose an environment file", pattern: "" }} label="Environment file" />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="compose-name">Project name (optional)</Label>
            <Input id="compose-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Defaults to the name in the file or the directory name" spellCheck={false} className="h-8" />
          </div>

          {validated && (
            <div role="status" className="flex items-center gap-1.5 rounded-md border border-success/30 bg-success/10 p-3 text-xs text-success">
              <CheckCircle2 className="size-3.5 shrink-0" aria-hidden />
              The Compose files are valid. Project {validated} saved{startAfter ? "; starting it…" : "."}
            </div>
          )}

          <label className="flex items-center gap-2 text-xs">
            <Checkbox checked={startAfter} onCheckedChange={(c) => setStartAfter(c === true)} />
            <span>
              Start the project after adding (<span className="font-mono">docker compose up -d</span>)
            </span>
          </label>

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
            <Button size="sm" disabled={!!blocked || add.isPending || !!validated} onClick={() => void submit()}>
              {add.isPending ? "Validating…" : startAfter ? "Validate, save and start" : "Validate and save"}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
