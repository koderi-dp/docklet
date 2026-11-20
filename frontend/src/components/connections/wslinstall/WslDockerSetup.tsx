import { useState, type ReactNode } from "react";
import { useMutation } from "@tanstack/react-query";
import { Browser } from "@wailsio/runtime";
import { Download, ExternalLink, Info, Loader2, Play } from "lucide-react";
import { toast } from "sonner";
import { ConnectionService } from "@bindings/services";
import { WSLInstallState } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { errorMessage, toastError } from "@/lib/errors";
import { cn } from "@/lib/utils";
import { InstallDockerDialog, type SuccessAction } from "./InstallDockerDialog";
import { useInstallOperation, useRefreshWslDocker, useWslPlan } from "./useWslInstall";

interface Props {
  distro: string;
  /** When the daemon already answers there is nothing to offer (the plan is not even requested). */
  daemonReachable?: boolean;
  /** Primary action once the install finished, for example "Test & Save". */
  successAction?: SuccessAction;
  className?: string;
}

function Panel({ tone = "default", title, children, actions }: { tone?: "default" | "warning"; title: string; children?: ReactNode; actions?: ReactNode }) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-3 rounded-md border p-3 text-sm",
        tone === "warning" ? "border-warning/40 bg-warning/10" : "bg-muted/30",
      )}
    >
      <div className="min-w-0 flex-1">
        <p className="font-medium">{title}</p>
        {children && <div className="mt-0.5 text-xs text-muted-foreground">{children}</div>}
      </div>
      {actions && <div className="flex shrink-0 gap-2">{actions}</div>}
    </div>
  );
}

/**
 * What to do when the selected WSL distribution has no running Docker Engine:
 * install it, start it, or (Docker Desktop's proxy, other distro families) explain why not.
 */
export function WslDockerSetup({ distro, daemonReachable, successAction, className }: Props) {
  const [dialog, setDialog] = useState<{ opId?: string } | null>(null);
  const runningOp = useInstallOperation(distro);
  const plan = useWslPlan(distro, !daemonReachable || !!dialog);
  const refresh = useRefreshWslDocker();
  const start = useMutation({
    mutationFn: () => ConnectionService.StartDockerInWSL(distro),
    onSuccess: (info) => {
      toast.success(`Docker Engine ${info.serverVersion} is running in ${distro}`);
      void refresh(distro);
    },
    onError: (e) => toastError(`Could not start Docker in ${distro}`, e),
  });
  const p = plan.data;

  const body = (() => {
    if (runningOp) {
      return (
        <Panel
          title={`Installing Docker Engine in ${distro}…`}
          actions={
            <Button size="sm" variant="outline" onClick={() => setDialog({ opId: runningOp.id })}>
              Show progress
            </Button>
          }
        >
          Running in the background. It is also listed in Operations.
        </Panel>
      );
    }
    if (daemonReachable) return null;
    if (plan.isLoading)
      return (
        <p className="flex items-center gap-2 text-xs text-muted-foreground">
          <Loader2 className="size-3.5 animate-spin" /> Inspecting {distro}…
        </p>
      );
    if (plan.error)
      return (
        <p role="alert" className="text-xs text-destructive">
          Could not inspect {distro}: {errorMessage(plan.error)}
        </p>
      );
    if (!p) return null;
    switch (p.state) {
      case WSLInstallState.WSLStateDockerDesktop:
        return (
          <Panel title="Docker comes from Docker Desktop here">
            <span className="flex gap-1.5">
              <Info className="mt-0.5 size-3.5 shrink-0" />
              {p.reason}
            </span>
          </Panel>
        );
      case WSLInstallState.WSLStateDaemonStopped:
        return (
          <Panel
            tone="warning"
            title={`Docker Engine is installed in ${distro} but not running`}
            actions={
              <Button size="sm" onClick={() => start.mutate()} disabled={start.isPending}>
                {start.isPending ? <Loader2 className="animate-spin" /> : <Play />}
                Start Docker Engine
              </Button>
            }
          >
            Starts the Docker service as root inside the distribution.
          </Panel>
        );
      case WSLInstallState.WSLStateNotInstalled:
        return p.supported ? (
          <Panel
            tone="warning"
            title={`Docker Engine is not installed in ${distro}`}
            actions={
              <Button size="sm" onClick={() => setDialog({})}>
                <Download />
                Install Docker Engine…
              </Button>
            }
          >
            Docklet can install it from Docker's official apt repository. You review every command first.
          </Panel>
        ) : (
          <Panel
            tone="warning"
            title={`Docker Engine is not installed in ${distro}`}
            actions={
              p.docsUrl ? (
                <Button size="sm" variant="outline" onClick={() => void Browser.OpenURL(p.docsUrl as string).catch((e) => toastError("Could not open the browser", e))}>
                  <ExternalLink />
                  Open install guide
                </Button>
              ) : undefined
            }
          >
            {p.reason}
          </Panel>
        );
      default:
        return null;
    }
  })();

  return (
    <>
      {body && <div className={className}>{body}</div>}
      {dialog && <InstallDockerDialog distro={distro} opId={dialog.opId} onClose={() => setDialog(null)} successAction={successAction} />}
    </>
  );
}
