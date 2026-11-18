import { useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { ContainerService } from "@bindings/services";
import type { ContainerRecreateInfo } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { containersKey } from "@/hooks/useContainers";
import { useOperations } from "@/hooks/useOperations";
import { errorMessage } from "@/lib/errors";
import { ContainerFormFields } from "../ContainerFormFields";
import { formFromSpec, specFromForm, validateForm, type FormState } from "../createContainerForm";
import { ComposeManagedPanel } from "./ComposeManagedPanel";
import { RecreateSummary } from "./RecreateSummary";
import { useRecreateInfo } from "./useRecreate";

interface Props {
  connectionId: string;
  containerId: string;
  onClose: () => void;
}

/** "Edit & recreate": edit the container's configuration, review the changes, then replace it safely. */
export function RecreateContainerDialog({ connectionId, containerId, onClose }: Props) {
  const q = useRecreateInfo(connectionId, containerId);
  const title = q.data ? `Edit & recreate ${q.data.name}` : "Edit & recreate";
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="flex max-h-[88vh] flex-col gap-3 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>Changes are applied by creating a replacement container; the original is only removed once the new one runs.</DialogDescription>
        </DialogHeader>
        {q.isLoading && <Skeleton className="h-48 w-full" />}
        {q.error && (
          <>
            <p role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-[13px] text-destructive">
              Could not read the container: {errorMessage(q.error)}
            </p>
            <DialogFooter>
              <Button variant="outline" size="sm" onClick={onClose}>Close</Button>
            </DialogFooter>
          </>
        )}
        {q.data && <Editor connectionId={connectionId} info={q.data} onClose={onClose} />}
      </DialogContent>
    </Dialog>
  );
}

function Editor({ connectionId, info, onClose }: { connectionId: string; info: ContainerRecreateInfo; onClose: () => void }) {
  const qc = useQueryClient();
  const { setCenterOpen } = useOperations();
  const [form, setForm] = useState<FormState>(() => formFromSpec(info.spec));
  const [copy, setCopy] = useState(false);
  const [step, setStep] = useState<"edit" | "confirm">("edit");
  const [pullFirst, setPullFirst] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const edited = useMemo(() => specFromForm(form, info.spec), [form, info.spec]);

  if (info.composeManaged && !copy) {
    return (
      <ComposeManagedPanel
        connectionId={connectionId}
        info={info}
        onClose={onClose}
        onCopy={() => {
          setCopy(true);
          setForm((f) => ({ ...f, name: `${info.name}-copy`, autoStart: false }));
        }}
      />
    );
  }

  const review = () => {
    const v = validateForm({ ...form });
    if (v) return setError(v);
    if (!form.name.trim()) return setError("A name is required.");
    if (copy && form.name.trim() === info.name) return setError("A copy needs a different name than the original.");
    setError(null);
    setStep("confirm");
  };

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      await ContainerService.Recreate(connectionId, info.id, edited, { pullFirst, keepOriginal: copy });
      void qc.invalidateQueries({ queryKey: containersKey(connectionId) });
      toast.info(copy ? `Copying ${info.name}…` : `Recreating ${info.name}…`, {
        description: "Progress is shown in the Activity center.",
        action: { label: "View activity", onClick: () => setCenterOpen(true) },
      });
      onClose();
    } catch (e) {
      setError(errorMessage(e));
      setStep("edit");
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
        {step === "edit" ? (
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault();
              review();
            }}
          >
            {copy && <p className="text-[13px] text-muted-foreground">Standalone copy: the Compose container is not changed. Choose a new name and review the settings.</p>}
            <ContainerFormFields form={form} onChange={(p) => setForm((f) => ({ ...f, ...p }))} edit nameLabel="Name *" />
            <button type="submit" className="hidden" aria-hidden tabIndex={-1} />
          </form>
        ) : (
          <RecreateSummary info={info} edited={edited} copy={copy} pullFirst={pullFirst} onPullFirst={setPullFirst} />
        )}
        {error && (
          <p role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-[13px] text-destructive">
            {error}
          </p>
        )}
      </div>
      <DialogFooter>
        <Button variant="outline" size="sm" onClick={step === "edit" ? onClose : () => setStep("edit")} disabled={busy}>
          {step === "edit" ? "Cancel" : "Back"}
        </Button>
        {step === "edit" ? (
          <Button size="sm" onClick={review} disabled={!form.image.trim()}>
            Review changes
          </Button>
        ) : (
          <Button size="sm" variant={copy ? "default" : "destructive"} onClick={() => void submit()} disabled={busy}>
            {busy && <Loader2 className="size-4 animate-spin" />}
            {copy ? "Create copy" : info.running ? "Stop and recreate" : "Recreate"}
          </Button>
        )}
      </DialogFooter>
    </>
  );
}
