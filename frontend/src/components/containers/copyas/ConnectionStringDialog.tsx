import { useState } from "react";
import { Copy, Info, KeyRound } from "lucide-react";
import { CopyAsService } from "@bindings/services";
import type { ConnectionEntry } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { copyToClipboard } from "@/lib/clipboard";
import { errorMessage, toastError } from "@/lib/errors";
import { SecretConfirm } from "./SecretConfirm";
import { useConnectionStrings } from "./useCopyAs";

interface Props {
  connectionId: string;
  containerId: string;
  name: string;
  onClose: () => void;
}

/** Connection strings of a database container, with a placeholder password unless explicitly asked for. */
export function ConnectionStringDialog({ connectionId, containerId, name, onClose }: Props) {
  const q = useConnectionStrings(connectionId, containerId);
  const [pending, setPending] = useState<number | null>(null);
  const d = q.data;

  const withPassword = async (index: number) => {
    setPending(null);
    try {
      const r = await CopyAsService.ConnectionStrings(connectionId, containerId, true);
      const e = r.entries?.[index];
      if (e) copyToClipboard(e.text, `${e.label} with password`);
    } catch (e) {
      toastError("Could not read the container", e);
    }
  };

  return (
    <>
      <Dialog open onOpenChange={(o) => !o && onClose()}>
        <DialogContent className="flex max-h-[88vh] flex-col gap-3 sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Connection string{d ? `: ${d.title}` : ""}</DialogTitle>
            <DialogDescription>
              {name}. Credentials come from the container's environment; the password is never filled in unless you ask for it.
            </DialogDescription>
          </DialogHeader>
          {q.isLoading && <Skeleton className="h-24 w-full" />}
          {q.error && (
            <p role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-[13px] text-destructive">
              {errorMessage(q.error)}
            </p>
          )}
          {d?.note && (
            <p className="flex items-start gap-2 rounded-md border border-border bg-muted/40 px-3 py-2 text-[13px]">
              <Info className="mt-0.5 size-4 shrink-0 text-info" />
              <span>{d.note}</span>
            </p>
          )}
          {d && (
            <ul className="flex min-h-0 flex-col gap-2 overflow-auto">
              {(d.entries ?? []).map((e, i) => (
                <EntryRow key={`${e.label}-${i}`} entry={e} canAskPassword={!!d.passwordEnv} onCopy={() => copyToClipboard(e.text, e.label)} onAskPassword={() => setPending(i)} />
              ))}
            </ul>
          )}
          <DialogFooter>
            <Button variant="outline" size="sm" onClick={onClose}>Close</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <SecretConfirm
        open={pending !== null}
        names={d?.passwordEnv ? [d.passwordEnv] : []}
        what="connection string"
        onCancel={() => setPending(null)}
        onConfirm={() => pending !== null && void withPassword(pending)}
      />
    </>
  );
}

function EntryRow({ entry, canAskPassword, onCopy, onAskPassword }: { entry: ConnectionEntry; canAskPassword: boolean; onCopy: () => void; onAskPassword: () => void }) {
  return (
    <li className="grid grid-cols-[88px_1fr_76px] items-start gap-2">
      <div className="pt-1.5 text-xs text-muted-foreground">
        {entry.label}
        {entry.internal && <span className="block text-[11px]">containers only</span>}
      </div>
      <code className="min-w-0 rounded-md border border-border bg-muted/40 px-2 py-1.5 font-mono text-xs break-all">{entry.text}</code>
      <div className="flex gap-1">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="outline" size="icon" className="size-8" aria-label={`Copy ${entry.label}`} onClick={onCopy}>
              <Copy className="size-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Copy{entry.usesPassword ? " (with placeholder)" : ""}</TooltipContent>
        </Tooltip>
        {entry.usesPassword && canAskPassword && (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="outline" size="icon" className="size-8" aria-label={`Copy ${entry.label} with password`} onClick={onAskPassword}>
                <KeyRound className="size-3.5" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Copy with the real password…</TooltipContent>
          </Tooltip>
        )}
      </div>
    </li>
  );
}
