import { useEffect, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, Loader2, OctagonX } from "lucide-react";
import { ConnectionService } from "@bindings/services";
import { OperationStatus, type Operation } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { errorMessage } from "@/lib/errors";
import { useOperationLogs, useOperations, useOpsStore } from "@/hooks/useOperations";
import { InstallConfirm, type InstallChoices } from "./InstallConfirm";
import { CopyLogButton, InstallProgress, LogTail } from "./InstallProgress";
import { useInstallOperation, useRefreshWslDocker, useWslPlan, wslInfoKey } from "./useWslInstall";

export interface SuccessAction {
  label: string;
  run: () => void;
}

export interface FlowHeader {
  title: string;
  description: string;
  /** confirm: nothing started yet; running; finished: succeeded, failed or cancelled. */
  phase: "confirm" | "running" | "finished";
}

interface FlowProps {
  distro: string;
  /** Attach to an install that is already running (the dialog was closed and reopened). */
  opId?: string;
  /** Cancel (before the install starts) and Close/Done. Omit it to hide those buttons. */
  onClose?: () => void;
  /** The primary button once Docker is running, for example "Test & Save" or "Continue". */
  successAction?: SuccessAction;
  /** Renders the title and description: a dialog header in the dialog, a plain heading in the setup wizard. */
  header: (h: FlowHeader) => ReactNode;
  /** The finished state needs no Done button because the surrounding screen has its own way forward. */
  inline?: boolean;
  className?: string;
}

/**
 * Confirm, run and report the Docker Engine install in one distribution. The same body is shown in
 * InstallDockerDialog and inline in the setup wizard. Leaving it never cancels a running install.
 */
export function InstallDockerFlow({ distro, opId, onClose, successAction, header, inline, className }: FlowProps) {
  const [startedId, setStartedId] = useState(opId);
  const op = useInstallOperation(distro, startedId);
  const title = op ? stateTitle(op, distro) : `Install Docker Engine in ${distro}?`;
  const description = !op
    ? "Review what Docklet will change in this distribution."
    : op.status === OperationStatus.OpRunning
      ? inline ? "Leaving this screen does not stop the install." : "Also listed in Operations. Closing this window does not stop the install."
      : inline
        ? "Copy the log if you need to report a problem."
        : "The full log is in Operations.";
  return (
    <div className={cn("flex flex-col gap-4", className)}>
      {header({ title, description, phase: !op ? "confirm" : op.status === OperationStatus.OpRunning ? "running" : "finished" })}
      {op ? (
        <RunView op={op} distro={distro} onClose={onClose} inline={inline} onRetry={() => setStartedId(undefined)} successAction={successAction} />
      ) : (
        <ConfirmView distro={distro} onClose={onClose} onStarted={(o) => setStartedId(o.id)} />
      )}
    </div>
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

function ConfirmView({ distro, onClose, onStarted }: { distro: string; onClose?: () => void; onStarted: (op: Operation) => void }) {
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
      <Footer>
        {onClose && (
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
        )}
        <Button onClick={start} disabled={!ready}>
          {busy && <Loader2 className="animate-spin" />}
          Install
        </Button>
      </Footer>
    </>
  );
}

function RunView({
  op,
  distro,
  onClose,
  inline,
  onRetry,
  successAction,
}: {
  op: Operation;
  distro: string;
  onClose?: () => void;
  inline?: boolean;
  onRetry: () => void;
  successAction?: SuccessAction;
}) {
  const { lines } = useOperationLogs(op.id, true);
  const { cancel, openOperations } = useOperations();
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
      <Footer className="gap-2 sm:justify-between">
        <div className="flex gap-2">
          {running && (
            <Button variant="outline" onClick={() => void cancel(op.id)}>
              <OctagonX />
              Cancel install
            </Button>
          )}
          {!running && <CopyLogButton lines={lines} />}
          {!inline && (
            <Button variant="ghost" onClick={() => openOperations(op.id)}>
              Open Operations
            </Button>
          )}
        </div>
        <div className="flex gap-2">
          {!running && !succeeded && (
            <Button variant="outline" onClick={onRetry}>
              Retry
            </Button>
          )}
          {onClose && !inline && (
            <Button variant={succeeded && successAction ? "outline" : "default"} onClick={onClose}>
              {running ? "Close" : "Done"}
            </Button>
          )}
          {succeeded && successAction && (
            <Button
              data-primary
              onClick={() => {
                onClose?.();
                successAction.run();
              }}
            >
              {successAction.label}
            </Button>
          )}
        </div>
      </Footer>
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

/** The button row under the flow (plain div: it is used in a dialog and inline). */
function Footer({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cn("flex flex-wrap items-center justify-end gap-2", className)}>{children}</div>;
}
