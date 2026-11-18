import { useQuery } from "@tanstack/react-query";
import { ConnectionService } from "@bindings/services";
import type { ConnectionProfile, PodmanMachineInfo } from "@bindings/model";
import { cn } from "@/lib/utils";
import { TypeIcon } from "./connectionMeta";

/**
 * Podman engines found on this computer (machine pipes, sockets), offered in the Add connection
 * dialog. A stopped machine is listed with the command that starts it; Docklet never starts one.
 */
export function DetectedPodman({ onPick }: { onPick: (profile: ConnectionProfile) => void }) {
  const { data } = useQuery({
    queryKey: ["podman-machines"],
    queryFn: () => ConnectionService.PodmanMachines(),
    staleTime: 15_000,
    retry: false,
  });
  const machines = (data ?? []).filter((m: PodmanMachineInfo) => m.profile.endpoint || !m.running);
  if (!machines.length) return null;
  return (
    <div className="space-y-1.5">
      <p className="text-xs text-muted-foreground">Podman engines detected on this computer</p>
      <div className="flex flex-wrap gap-2">
        {machines.map((m) => {
          const usable = m.running && !!m.profile.endpoint;
          return (
            <button
              key={m.profile.id || m.name}
              type="button"
              disabled={!usable}
              title={usable ? "Fill in this connection" : m.hint}
              onClick={() => onPick({ ...m.profile, id: "", discovered: false })}
              className={cn(
                "flex items-center gap-2 rounded-md border bg-card px-2.5 py-1.5 text-left text-sm transition-colors",
                usable ? "hover:bg-muted" : "cursor-not-allowed opacity-70",
              )}
            >
              <TypeIcon type={m.profile.type} className="size-4 text-muted-foreground" />
              <span className="flex flex-col">
                <span className="font-medium">{m.profile.name}</span>
                <span className="text-[11px] leading-tight text-muted-foreground">{usable ? "Running" : `Stopped. ${m.hint ?? ""}`.trim()}</span>
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
