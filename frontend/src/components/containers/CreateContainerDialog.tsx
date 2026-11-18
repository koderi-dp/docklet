import { useEffect, useMemo, useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { ContainerService } from "@bindings/services";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useConnections } from "@/hooks/useActiveConnection";
import { PortErrorHint, portAwareDescription } from "@/components/ports/PortErrorHint";
import { usePortChecks } from "@/hooks/usePortChecks";
import { usePortSubmitGuard } from "@/components/ports/usePortSubmitGuard";
import { containersKey } from "@/hooks/useContainers";
import { errorMessage } from "@/lib/errors";
import { ContainerFormFields } from "./ContainerFormFields";
import { emptyForm, specFromForm, validateForm, type FormState } from "./createContainerForm";

export interface CreateContainerDialogProps {
  connectionId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Optional image reference to prefill. */
  image?: string;
}

export default function CreateContainerDialog({ connectionId, open, onOpenChange, image: imageProp }: CreateContainerDialogProps) {
  const qc = useQueryClient();
  const { data: profiles } = useConnections();
  const engineName = profiles?.find((p) => p.id === connectionId)?.name ?? connectionId;

  const [form, setForm] = useState<FormState>(() => emptyForm(imageProp));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Fresh form each time the dialog opens.
  useEffect(() => {
    if (!open) return;
    setForm(emptyForm(imageProp));
    setError(null);
    setBusy(false);
  }, [open, imageProp]);

  const clientError = useMemo(() => validateForm({ ...form }), [form]);
  const portChecks = usePortChecks(connectionId, form.ports);
  const guard = usePortSubmitGuard({
    connectionId,
    rows: form.ports,
    setRows: (ports) => setForm((f) => ({ ...f, ports })),
    anywayLabel: "Create anyway",
    what: "container",
  });

  const submit = async () => {
    if (clientError) {
      setError(clientError);
      return;
    }
    if (!(await guard.confirm())) return;
    setBusy(true);
    setError(null);
    try {
      const res = await ContainerService.Create(connectionId, specFromForm(form));
      void qc.invalidateQueries({ queryKey: containersKey(connectionId) });
      if (res.error) {
        toast.warning(`Container ${res.name} created`, { description: portAwareDescription(connectionId, res.error), duration: Infinity, closeButton: true });
      } else {
        toast.success(`Container ${res.name} ${form.autoStart ? "created and started" : "created"}`);
      }
      onOpenChange(false);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
    <Dialog open={open} onOpenChange={(o) => !busy && onOpenChange(o)}>
      <DialogContent className="flex max-h-[88vh] flex-col gap-3 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>New container</DialogTitle>
          <DialogDescription>
            Creates a container on <span className="font-medium text-foreground">{engineName}</span>. The image must already exist on
            this Engine; it is not pulled automatically.
          </DialogDescription>
        </DialogHeader>

        <form
          className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1"
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <ContainerFormFields form={form} onChange={(p) => setForm((f) => ({ ...f, ...p }))} portChecks={portChecks} />
          {error && (
            <div role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-[13px] text-destructive">
              <p>{error}</p>
              <PortErrorHint connectionId={connectionId} message={error} />
            </div>
          )}
          <button type="submit" className="hidden" aria-hidden tabIndex={-1} />
        </form>

        <DialogFooter>
          <Button variant="outline" size="sm" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancel
          </Button>
          <Button size="sm" onClick={() => void submit()} disabled={busy || !form.image.trim()}>
            {busy && <Loader2 className="size-4 animate-spin" />}
            {busy ? "Creating…" : "Create container"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
    {guard.dialog}
    </>
  );
}
