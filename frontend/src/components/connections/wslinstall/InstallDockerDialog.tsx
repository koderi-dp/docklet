import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, Loader2, OctagonX } from "lucide-react";
import { ConnectionService } from "@bindings/services";
import { OperationStatus, type Operation } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { errorMessage } from "@/lib/errors";
import { useOperationLogs, useOperations, useOpsStore } from "@/hooks/useOperations";
import { InstallConfirm, type InstallChoices } from "./InstallConfirm";
import { CopyLogButton, InstallProgress, LogTail } from "./InstallProgress";
import { useInstallOperation, useRefreshWslDocker, useWslPlan, wslInfoKey } from "./useWslInstall";

export interface SuccessAction {
  label: string;
  run: () => void;
}

interface Props {
  distro: string;
  /** Attach to an install that is already running (the dialog was closed and reopened). */
  opId?: string;
  onClose: () => void;
  /** The primary button once Docker is running, for example "Test & Save". */
  successAction?: SuccessAction;
}

/** Confirm, run and report the Docker Engine install in one distribution. Closing it never cancels the install. */
export function InstallDockerDialog({ distro, opId, onClose, successAction }: Props) {
  const [startedId, setStartedId] = useState(opId);
  const op = useInstallOperation(distro, startedId);
  const title = op ? stateTitle(op, distro) : `Install Docker Engine in ${distro}?`;
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90vh] gap-4 overflow-y-auto sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {!op
              ? "Review what Docklet will change in this distribution."
              : op.status === OperationStatus.OpRunning
                ? "Also listed in the Activity center. Closing this window does not stop the install."
                : "The full log is in the Activity center."}
          </DialogDescription>
        </DialogHeader>
        {op ? (
          <RunView op={op} distro={distro} onClose={onClose} onRetry={() => setStartedId(undefined)} successAction={successAction} />
        ) : (
          <ConfirmView distro={distro} onClose={onClose} onStarted={(o) => setStartedId(o.id)} />
        )}
      </DialogContent>
    </Dialog>
  );
}

function stateTitle(op: Operation, distro: string): string {
  switch (op.status) {
    case OperationStatus.OpRunning:
      return `Installing Docker Engine in ${distro}`;
    case OperationStatus.OpSucceeded:
      return "Docker Engine installed";
    case OperationStatus.OpCancelled:
      return "Install cancelled";
    default:
      return "Install failed";
  }
}

function ConfirmView({ distro, onClose, onStarted }: { distro: string; onClose: () => void; onStarted: (op: Operation) => void }) {
  const plan = useWslPlan(distro);
  const [choices, setChoices] = useState<InstallChoices>({ removeConflicting: false, enableSystemd: false });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const p = plan.data;
  const needsRemoval = (p?.packagesToRemove ?? []).length > 0;
  const ready = !!p?.supported && (!needsRemoval || choices.removeConflicting) && !busy;

  const start = async () => {
    setBusy(true);
    setError(null);
    try {
      const op = await ConnectionService.InstallDockerInWSL(distro, choices);
      useOpsStore.getState().upsert(op);
      onStarted(op);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      {plan.isLoading && <Skeleton className="h-40 w-full" />}
      {plan.error && <ErrorLine text={`Could not inspect ${distro}: ${errorMessage(plan.error)}`} />}
      {p && !p.supported && <ErrorLine text={p.reason || "Docker Engine cannot be installed here automatically."} />}
      {p?.supported && <InstallConfirm plan={p} choices={choices} onChange={setChoices} />}
      {error && <ErrorLine text={error} />}
      <DialogFooter>
        <Button variant="outline" onClick={onClose}>
          Cancel
        </Button>
        <Button onClick={start} disabled={!ready}>
          {busy && <Loader2 className="animate-spin" />}
          Install
        </Button>
      </DialogFooter>
    </>
  );
}

function RunView({
  op,
  distro,
  onClose,
  onRetry,
  successAction,
}: {
  op: Operation;
  distro: string;
  onClose: () => void;
  onRetry: () => void;
  successAction?: SuccessAction;
}) {
  const { lines } = useOperationLogs(op.id, true);
  const { cancel, setCenterOpen } = useOperations();
  const refresh = useRefreshWslDocker();
  const running = op.status === OperationStatus.OpRunning;
  const succeeded = op.status === OperationStatus.OpSucceeded;

  useEffect(() => {
    if (!running) void refresh(distro);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [running, distro]);

  return (
    <>
      {running && <InstallProgress op={op} lines={lines} />}
      {succeeded && <Succeeded distro={distro} />}
      {!running && !succeeded && (
        <div className="space-y-2">
          <ErrorLine
            text={
              op.status === OperationStatus.OpCancelled
                ? "The install was cancelled. Steps that already finished were not undone."
                : op.error || "The install failed."
            }
          />
          <LogTail lines={lines.slice(-30)} />
        </div>
      )}
      <DialogFooter className="gap-2 sm:justify-between">
        <div className="flex gap-2">
          {running && (
            <Button variant="outline" onClick={() => void cancel(op.id)}>
              <OctagonX />
              Cancel install
            </Button>
          )}
          {!running && <CopyLogButton lines={lines} />}
          <Button variant="ghost" onClick={() => setCenterOpen(true)}>
            Open Activity
          </Button>
        </div>
        <div className="flex gap-2">
          {!running && !succeeded && (
            <Button variant="outline" onClick={onRetry}>
              Retry
            </Button>
          )}
          <Button variant={succeeded && successAction ? "outline" : "default"} onClick={onClose}>
            {running ? "Close" : "Done"}
          </Button>
          {succeeded && successAction && (
            <Button
              onClick={() => {
                onClose();
                successAction.run();
              }}
            >
              {successAction.label}
            </Button>
          )}
        </div>
      </DialogFooter>
    </>
  );
}

function Succeeded({ distro }: { distro: string }) {
  const info = useQuery({ queryKey: wslInfoKey(distro), queryFn: () => ConnectionService.WSLDockerInfo(distro), retry: false });
  const ok = info.data?.daemonReachable;
  return (
    <p
      role="status"
      className={`flex items-center gap-2 rounded-md border p-3 text-sm ${ok ? "border-success/30 bg-success/10 text-success" : "bg-muted/40 text-muted-foreground"}`}
    >
      {info.isLoading ? <Loader2 className="size-4 shrink-0 animate-spin" /> : <CheckCircle2 className="size-4 shrink-0" />}
      {info.isLoading
        ? "Checking Docker…"
        : ok
          ? `Docker Engine ${info.data?.serverVersion} is running in ${distro}.`
          : (info.data?.detail ?? "Installed. Docker has not answered yet.")}
    </p>
  );
}

function ErrorLine({ text }: { text: string }) {
  return (
    <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm break-words text-destructive">
      {text}
    </p>
  );
}
