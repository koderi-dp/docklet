import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { ComposeService, ImageService } from "@bindings/services";
import { OperationStatus, type ContainerRecreateInfo } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { awaitOperation } from "@/lib/awaitOperation";
import { errorMessage } from "@/lib/errors";

interface Props {
  connectionId: string;
  info: ContainerRecreateInfo;
  onCopy: () => void;
  onClose: () => void;
}

/** Explains why a Compose container is not replaced in place and offers the correct paths. */
export function ComposeManagedPanel({ connectionId, info, onCopy, onClose }: Props) {
  const project = info.composeProject ?? "";
  const service = info.composeService ?? "";
  const [busy, setBusy] = useState<string | null>(null);
  const cap = useQuery({
    queryKey: ["compose-capability", connectionId, project],
    queryFn: () => ComposeService.Capability(connectionId, project),
    retry: false,
  });
  const canRun = cap.data?.canRun === true;
  const why = cap.isLoading ? "Checking Compose…" : cap.error ? errorMessage(cap.error) : cap.data?.reason || "Compose cannot run this project here.";

  const updateWithCompose = async () => {
    try {
      setBusy(`Pulling ${info.spec.image}…`);
      const pull = await awaitOperation((await ImageService.Pull(connectionId, info.spec.image)).id);
      if (pull.status !== OperationStatus.OpSucceeded) {
        toast.error("Pull failed", { description: pull.error || "The image was not updated.", duration: Infinity, closeButton: true });
        return;
      }
      await ComposeService.UpService(connectionId, project, service, { build: false, removeOrphans: false, profiles: null });
      toast.info(`Compose is updating ${service}`, { description: "Follow the progress in Operations." });
      onClose();
    } catch (e) {
      toast.error("Could not update with Compose", { description: errorMessage(e), duration: Infinity, closeButton: true });
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-3 text-[13px]">
      <p className="rounded-md border border-warning/40 bg-warning/10 px-3 py-2">
        <span className="font-medium">{info.name}</span> is managed by Docker Compose (project <span className="font-mono">{project}</span>, service{" "}
        <span className="font-mono">{service}</span>). Replacing it here would make the container differ from the Compose file, and the next{" "}
        <span className="font-mono">compose up</span> would undo your change.
      </p>
      <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
        <li>To change its configuration, edit the Compose file and run Up for the project.</li>
        <li>To run a newer image, pull it and bring the service up again with Compose.</li>
        <li>Or create a standalone copy under a new name; the Compose container stays as it is.</li>
      </ul>
      <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
        <Button variant="outline" size="sm" onClick={onClose} disabled={!!busy}>
          Cancel
        </Button>
        <Button variant="outline" size="sm" onClick={onCopy} disabled={!!busy}>
          Create standalone copy…
        </Button>
        <Button size="sm" onClick={() => void updateWithCompose()} disabled={!canRun || !!busy} title={canRun ? "Pull the image, then compose up for this service" : why}>
          {busy && <Loader2 className="size-4 animate-spin" />}
          {busy ?? "Update image with Compose"}
        </Button>
      </div>
    </div>
  );
}
