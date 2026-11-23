import { FolderOpen, Info, X } from "lucide-react";
import { toast } from "sonner";
import { DialogService } from "@bindings/services";
import type { ComposeLocation } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { errorMessage } from "@/lib/errors";

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

export type PathPick = { kind: "file"; title: string; pattern: string } | { kind: "folder"; title: string };

/** One path row: a text field (typed or picked) and a native picker button for a file or a folder. */
export function PathRow({
  value,
  onChange,
  onRemove,
  placeholder,
  label,
  pick,
}: {
  value: string;
  onChange: (v: string) => void;
  onRemove?: () => void;
  placeholder: string;
  label: string;
  pick: PathPick;
}) {
  const browse = async () => {
    try {
      const p = pick.kind === "folder" ? await DialogService.PickDirectory(pick.title) : await DialogService.PickFile(pick.title, label, pick.pattern);
      if (p) onChange(p);
    } catch (e) {
      toast.error(pick.kind === "folder" ? "Could not open the folder dialog" : "Could not open the file dialog", { description: errorMessage(e) });
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
