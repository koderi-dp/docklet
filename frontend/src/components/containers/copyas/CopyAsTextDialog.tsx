import { useState } from "react";
import { CopyAsService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { copyToClipboard } from "@/lib/clipboard";
import { errorMessage, toastError } from "@/lib/errors";
import { useUI, type CopyAsShell } from "@/stores/uiStore";
import { SecretConfirm } from "./SecretConfirm";
import { useCopyAsText } from "./useCopyAs";

interface Props {
  kind: "run" | "compose";
  connectionId: string;
  containerId: string;
  name: string;
  onClose: () => void;
}

/** Preview of the generated `docker run` command or Compose service; copy it as is or with secret values. */
export function CopyAsTextDialog({ kind, connectionId, containerId, name, onClose }: Props) {
  const shell = useUI((s) => s.copyAsShell);
  const setShell = useUI((s) => s.setCopyAsShell);
  const q = useCopyAsText(kind, connectionId, containerId, shell);
  const [confirmSecrets, setConfirmSecrets] = useState(false);
  const what = kind === "run" ? "Docker run command" : "Compose service";
  const secretKeys = q.data?.secretKeys ?? [];

  const copy = () => {
    if (!q.data) return;
    copyToClipboard(q.data.text, what);
    onClose();
  };
  const copyWithSecrets = async () => {
    setConfirmSecrets(false);
    try {
      const r =
        kind === "run"
          ? await CopyAsService.DockerRun(connectionId, containerId, shell, true)
          : await CopyAsService.Compose(connectionId, containerId, true);
      copyToClipboard(r.text, `${what} with secret values`);
      onClose();
    } catch (e) {
      toastError("Could not read the container", e);
    }
  };

  return (
    <>
      <Dialog open onOpenChange={(o) => !o && onClose()}>
        <DialogContent className="flex max-h-[88vh] flex-col gap-3 sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>Copy {name} as {what.toLowerCase()}</DialogTitle>
            <DialogDescription>Only settings that differ from the image and Docker defaults are included.</DialogDescription>
          </DialogHeader>
          {kind === "run" && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground">Shell</span>
              <Tabs value={shell} onValueChange={(v) => setShell(v as CopyAsShell)}>
                <TabsList className="h-8">
                  <TabsTrigger value="bash" className="px-3 text-xs">bash / zsh</TabsTrigger>
                  <TabsTrigger value="powershell" className="px-3 text-xs">PowerShell</TabsTrigger>
                </TabsList>
              </Tabs>
            </div>
          )}
          {q.isLoading && <Skeleton className="h-48 w-full" />}
          {q.error && (
            <p role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-[13px] text-destructive">
              Could not read the container: {errorMessage(q.error)}
            </p>
          )}
          {q.data && (
            <>
              <pre
                aria-label={what}
                tabIndex={0}
                className="min-h-0 flex-1 overflow-auto rounded-md border border-border bg-muted/40 p-3 font-mono text-xs leading-relaxed whitespace-pre focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none"
              >
                {q.data.text}
              </pre>
              {secretKeys.length > 0 && (
                <p className="text-xs text-muted-foreground">
                  {secretKeys.length === 1 ? "1 secret-looking variable is" : `${secretKeys.length} secret-looking variables are`} written as a
                  variable reference instead of its value: <span className="font-mono text-foreground">{secretKeys.join(", ")}</span>
                </p>
              )}
            </>
          )}
          <DialogFooter>
            <Button variant="ghost" size="sm" onClick={onClose}>Close</Button>
            {secretKeys.length > 0 && (
              <Button variant="outline" size="sm" onClick={() => setConfirmSecrets(true)}>Copy with secret values…</Button>
            )}
            <Button size="sm" disabled={!q.data} onClick={copy}>Copy</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <SecretConfirm open={confirmSecrets} names={secretKeys} what={what.toLowerCase()} onCancel={() => setConfirmSecrets(false)} onConfirm={() => void copyWithSecrets()} />
    </>
  );
}
