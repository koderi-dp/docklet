import { AlertTriangle } from "lucide-react";
import type { WSLInstallPlan } from "@bindings/model";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { CommandList } from "./CommandList";

export interface InstallChoices {
  removeConflicting: boolean;
  enableSystemd: boolean;
}

/** Lists exactly what the install will do before anything runs. */
export function InstallConfirm({
  plan,
  choices,
  onChange,
}: {
  plan: WSLInstallPlan;
  choices: InstallChoices;
  onChange: (c: InstallChoices) => void;
}) {
  const remove = plan.packagesToRemove ?? [];
  const steps = [...(plan.steps ?? []), ...(choices.enableSystemd ? (plan.systemdSteps ?? []) : [])];
  const canEnableSystemd = !plan.systemd && (plan.systemdSteps ?? []).length > 0;
  return (
    <div className="space-y-3 text-sm">
      <div>
        <p className="font-medium">What will be installed</p>
        <p className="mt-1 text-muted-foreground">
          Docker CE, the Docker CLI, containerd, and the Buildx and Compose plugins, from Docker's official apt repository.
        </p>
      </div>

      {remove.length > 0 && (
        <div className="rounded-md border border-warning/40 bg-warning/10 p-3">
          <p className="flex items-center gap-2 font-medium text-warning">
            <AlertTriangle className="size-4 shrink-0" />
            Conflicting packages will be removed
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            These are installed in {plan.distro} and conflict with Docker's own packages. Docker's install guide removes them first.
          </p>
          <p className="mt-1.5 flex flex-wrap gap-1.5">
            {remove.map((p) => (
              <code key={p} className="rounded bg-background/60 px-1.5 py-0.5 font-mono text-xs">
                {p}
              </code>
            ))}
          </p>
          <div className="mt-2 flex items-center gap-2">
            <Checkbox
              id="wsl-remove"
              checked={choices.removeConflicting}
              onCheckedChange={(v) => onChange({ ...choices, removeConflicting: v === true })}
            />
            <Label htmlFor="wsl-remove" className="text-xs font-normal">
              Remove these packages (required to continue)
            </Label>
          </div>
        </div>
      )}

      <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
        {plan.user ? (
          <li>
            Add <span className="font-medium text-foreground">{plan.user}</span> to the <code className="font-mono text-xs">docker</code> group, so Docker works without sudo.
          </li>
        ) : (
          <li>The default user is root, so no group change is needed.</li>
        )}
        {plan.daemonStart === "systemd" ? (
          <li>
            Start Docker with <code className="font-mono text-xs">systemctl enable --now docker</code>: it also starts at every boot.
          </li>
        ) : (
          <li>
            Start Docker with <code className="font-mono text-xs">service docker start</code>. systemd is not running here, so after WSL restarts Docker has to be started again
            (Docklet offers Start Docker Engine).
          </li>
        )}
        <li>
          Check <code className="font-mono text-xs">docker version</code> and <code className="font-mono text-xs">docker info</code>. No image is pulled.
        </li>
      </ul>

      {canEnableSystemd && (
        <div className="rounded-md border bg-muted/30 p-3">
          <div className="flex items-center gap-2">
            <Checkbox
              id="wsl-systemd"
              checked={choices.enableSystemd}
              onCheckedChange={(v) => onChange({ ...choices, enableSystemd: v === true })}
            />
            <Label htmlFor="wsl-systemd" className="font-medium">
              Enable systemd in this distribution (restarts the distribution)
            </Label>
          </div>
          <p className="mt-1.5 pl-6 text-xs text-muted-foreground">
            Adds <code className="font-mono">systemd=true</code> to <code className="font-mono">/etc/wsl.conf</code> (other settings are kept) and runs{" "}
            <code className="font-mono">wsl.exe --terminate {plan.distro}</code>. That stops everything running in {plan.distro}: open shells, containers and servers.
            Docker then starts under systemd.
          </p>
        </div>
      )}

      <CommandList steps={steps} />

      <p className="text-xs text-muted-foreground">
        Commands run as root inside {plan.distro} through <code className="font-mono">wsl.exe</code>; no password is needed. Nothing is installed on Windows.
      </p>
    </div>
  );
}
