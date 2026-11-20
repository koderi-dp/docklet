import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import type { ImageRow } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { usePushImage, useRegistryAuthStatus } from "@/hooks/useImages";
import { useOperations } from "@/hooks/useOperations";
import { useSignInDialog } from "@/hooks/useRegistries";
import { errorMessage } from "@/lib/errors";

interface Props {
  connectionId: string;
  engineName: string;
  image: ImageRow | null;
  onClose: () => void;
}

/** Registries on this machine are commonly unauthenticated, so they stay pushable without credentials. */
function isLoopbackRegistry(registry: string): boolean {
  return /^(localhost|127\.0\.0\.1|\[::1\])(:\d+)?$/.test(registry);
}

export function describeSource(source: string): string {
  if (source === "config") return "Docker config file";
  if (source === "docklet") return "Docklet sign-in";
  if (source.startsWith("helper:")) return `credential helper "${source.slice(7)}"`;
  return source;
}

export function PushImageDialog({ connectionId, engineName, image, onClose }: Props) {
  const [serverError, setServerError] = useState<string | null>(null);
  const push = usePushImage(connectionId);
  const openSignIn = useSignInDialog((s) => s.open);
  const { openOperations } = useOperations();
  const ref = image && !image.dangling ? image.ref : null;
  const status = useRegistryAuthStatus(connectionId, ref);

  useEffect(() => {
    setServerError(null);
  }, [image?.key]);

  const hasCreds = status.data?.hasCredentials ?? false;
  const loopback = status.data ? isLoopbackRegistry(status.data.registry) : false;
  const canPush = !!ref && !!status.data && (hasCreds || loopback);
  const registry = status.data?.registry;

  const submit = async () => {
    if (!ref) return;
    setServerError(null);
    try {
      const op = await push.mutateAsync(ref);
      onClose();
      toast.success(`Push started: ${op.resource}`, {
        description: "Progress is shown in Operations.",
        action: { label: "View", onClick: () => openOperations(op.id) },
      });
    } catch (e) {
      setServerError(errorMessage(e));
    }
  };

  const pushButton = (
    <Button type="button" disabled={!canPush || push.isPending} onClick={() => void submit()}>
      {push.isPending && <Loader2 className="size-4 animate-spin" />}
      Push
    </Button>
  );

  return (
    <Dialog open={!!image} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Push image</DialogTitle>
          <DialogDescription>
            Upload <span className="font-mono text-foreground">{image?.ref}</span> from <span className="font-medium text-foreground">{engineName}</span> to its
            registry. Closing this dialog does not cancel the push.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-2 text-[13px]">
          {status.isLoading ? (
            <p className="text-muted-foreground">Checking Docker credentials…</p>
          ) : status.error ? (
            <p className="text-destructive">{String(status.error)}</p>
          ) : status.data ? (
            <p>
              Registry <span className="font-mono">{status.data.registry}</span>:{" "}
              {hasCreds ? (
                <span className="text-success">credentials found ({describeSource(status.data.source)})</span>
              ) : loopback ? (
                <span className="text-muted-foreground">no credentials found; local registries often need none</span>
              ) : (
                <span className="text-warning">no credentials found</span>
              )}
            </p>
          ) : null}
          {status.data && !hasCreds && !loopback && (
            <p className="flex flex-wrap items-center gap-2 text-muted-foreground">
              <Button type="button" size="sm" variant="outline" onClick={() => openSignIn({ registry })}>
                Sign in to {registry}
              </Button>
              <span>Credentials are saved on this computer. Existing docker login entries are used too.</span>
            </p>
          )}
          {serverError && <p className="whitespace-pre-wrap text-destructive">{serverError}</p>}
        </div>
        <DialogFooter className="pt-2">
          <Button type="button" variant="outline" onClick={onClose}>
            Cancel
          </Button>
          {canPush ? (
            pushButton
          ) : (
            <Tooltip>
              <TooltipTrigger asChild>
                <span>{pushButton}</span>
              </TooltipTrigger>
              <TooltipContent>
                {status.isLoading ? "Checking credentials" : registry ? `No credentials for ${registry}. Sign in first.` : "Not available"}
              </TooltipContent>
            </Tooltip>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
