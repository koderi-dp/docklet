import { FolderOpen, Info, X } from "lucide-react";
import { toast } from "sonner";
import { ComposeService } from "@bindings/services";
import type { ComposeLocation } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { errorMessage } from "@/lib/errors";
import { onWindowsDrive } from "@/lib/newComposeFile";

/** Where the Compose CLI runs for the connection, and how paths must be written there. */
export function ComposeLocationBox({ location: l, error }: { location: ComposeLocation | undefined; error?: unknown }) {
  return (
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
      {!!error && <p className="mt-1 text-destructive">{errorMessage(error)}</p>}
    </div>
  );
}

/** What to expect from a path on a Windows drive when Compose runs inside WSL; nothing for every other path. */
export function WindowsDriveNote({ location, paths }: { location: ComposeLocation | undefined; paths: readonly string[] }) {
  if (location?.kind !== "wsl" || !paths.some(onWindowsDrive)) return null;
  return (
    <p className="text-xs text-muted-foreground">
      This path is on a Windows drive. It works, but WSL reads such files more slowly and <span className="font-mono">docker compose watch</span> can miss changes there.
    </p>
  );
}

export type PathPick = { kind: "file"; title: string; pattern: string } | { kind: "folder"; title: string };

/**
 * One path row: a text field (typed or picked) and a native picker button for a file or a folder.
 * The picker belongs to the connection: for WSL the picked path comes back as the Linux path inside the distribution.
 */
export function PathRow({
  connectionId,
  value,
  onChange,
  onRemove,
  placeholder,
  label,
  pick,
}: {
  connectionId: string;
  value: string;
  onChange: (v: string) => void;
  onRemove?: () => void;
  placeholder: string;
  label: string;
  pick: PathPick;
}) {
  const browse = async () => {
    try {
      const p = pick.kind === "folder" ? await ComposeService.PickPath(connectionId, "folder", pick.title, "", "") : await ComposeService.PickPath(connectionId, "file", pick.title, label, pick.pattern);
      if (p) onChange(p);
    } catch (e) {
      toast.error(pick.kind === "folder" ? "Could not choose the folder" : "Could not choose the file", { description: errorMessage(e) });
    }
  };
  return (
    <div className="flex items-center gap-1.5">
      <Input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} spellCheck={false} className="h-8 font-mono text-xs" aria-label={label} />
      <Button type="button" size="sm" variant="outline" onClick={() => void browse()}>
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
