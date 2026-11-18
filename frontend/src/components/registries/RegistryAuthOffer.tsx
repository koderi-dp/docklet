import { KeyRound } from "lucide-react";
import { toast } from "sonner";
import { ImageService } from "@bindings/services";
import { OperationStatus, type Operation } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { useSignInDialog } from "@/hooks/useRegistries";
import { toastError } from "@/lib/errors";
import { authFailureHost } from "@/lib/registryPresets";

/** Offered under a failed pull or push that the registry refused for lack of (valid) credentials. */
export function RegistryAuthOffer({ op }: { op: Operation }) {
  const openSignIn = useSignInDialog((s) => s.open);
  const pull = op.kind === "image.pull";
  if (op.status !== OperationStatus.OpFailed || (!pull && op.kind !== "image.push")) return null;
  const host = authFailureHost(op.error);
  if (!host) return null;

  const retry = async () => {
    try {
      const started = await (pull ? ImageService.Pull(op.connectionId, op.resource) : ImageService.Push(op.connectionId, op.resource));
      toast.success(`${pull ? "Pull" : "Push"} started: ${started.resource}`);
    } catch (e) {
      toastError(`Could not ${pull ? "pull" : "push"} ${op.resource}`, e);
    }
  };

  return (
    <Button
      size="sm"
      variant="outline"
      className="mt-1.5 h-6 gap-1 px-2 text-xs"
      onClick={() =>
        openSignIn({
          registry: host,
          onSignedIn: () =>
            toast(`Retry ${pull ? "pull" : "push"} of ${op.resource}?`, { duration: Infinity, closeButton: true, action: { label: "Retry", onClick: () => void retry() } }),
        })
      }
    >
      <KeyRound className="size-3" />
      Sign in to {host}
    </Button>
  );
}
