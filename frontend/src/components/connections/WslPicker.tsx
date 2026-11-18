import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Play, RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { ConnectionService } from "@bindings/services";
import type { WSLDistro, WSLDockerInfo } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { errorMessage } from "@/lib/errors";
import { wslInfoKey } from "./wslinstall/useWslInstall";
import { WslDockerSetup } from "./wslinstall/WslDockerSetup";
import type { SuccessAction } from "./wslinstall/InstallDockerDialog";

const distrosKey = ["wsl-distros"] as const;

function isInternal(name: string) {
  const n = name.toLowerCase();
  return n.startsWith("docker-desktop") || n.startsWith("podman-");
}

function DockerLine({ d, info, loading }: { d: WSLDistro; info?: WSLDockerInfo; loading: boolean }) {
  if (loading) return <span className="text-muted-foreground">Checking Docker…</span>;
  if (!info) return <span className="text-muted-foreground">Docker status not available</span>;
  if (!info.running) return <span className="text-muted-foreground">Stopped: start the distribution to detect Docker</span>;
  if (info.daemonReachable)
    return <span className="text-success">Docker Engine {info.serverVersion} is running</span>;
  if (info.cliFound) return <span className="text-warning">Docker CLI found, daemon not running</span>;
  if (isInternal(d.name))
    return <span className="text-muted-foreground">{d.name.toLowerCase().startsWith("podman-") ? "Podman machine: use its Podman connection" : "Internal Docker Desktop distribution"}</span>;
  return <span className="text-muted-foreground">Docker not found in this distribution</span>;
}

/** WSL distribution picker with per-distro Docker detection and an explicit "Start WSL" action. */
export function WslPicker({ value, onChange, successAction }: { value: string; onChange: (distro: string) => void; successAction?: SuccessAction }) {
  const qc = useQueryClient();
  const distros = useQuery({ queryKey: distrosKey, queryFn: () => ConnectionService.WSLDistros(), retry: false, staleTime: 0 });
  const list = distros.data ?? [];
  const infos = useQueries({
    queries: list.map((d) => ({
      queryKey: wslInfoKey(d.name),
      queryFn: () => ConnectionService.WSLDockerInfo(d.name),
      retry: false,
      staleTime: 0,
    })),
  });
  const start = useMutation({
    mutationFn: (name: string) => ConnectionService.StartWSLDistro(name),
    onSuccess: (_r, name) => {
      toast.success(`${name} started`);
      void qc.invalidateQueries({ queryKey: distrosKey });
      void qc.invalidateQueries({ queryKey: wslInfoKey(name) });
    },
    onError: (e, name) =>
      toast.error(`Could not start ${name}`, {
        description: errorMessage(e),
        duration: Infinity,
        closeButton: true,
      }),
  });
  const selectedInfo = list.some((d) => d.name === value) ? infos[list.findIndex((d) => d.name === value)]?.data : undefined;
  const refresh = () => {
    void qc.invalidateQueries({ queryKey: distrosKey });
    void qc.invalidateQueries({ queryKey: ["wsl-docker"] });
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">Installed WSL distributions</span>
        <Button variant="ghost" size="xs" onClick={refresh} disabled={distros.isFetching}>
          <RefreshCw className={cn(distros.isFetching && "animate-spin")} />
          Refresh
        </Button>
      </div>
      {distros.isLoading && <Skeleton className="h-14 w-full" />}
      {distros.error && (
        <p className="text-sm text-destructive" role="alert">
          Could not list WSL distributions: {String(distros.error)}
        </p>
      )}
      {!distros.isLoading && !distros.error && list.length === 0 && (
        <p className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
          No WSL distributions found. WSL is only available on Windows, with at least one distribution installed.
        </p>
      )}
      <div className="space-y-1.5" role="radiogroup" aria-label="WSL distribution">
        {list.map((d, i) => {
          const q = infos[i];
          const info = q?.data;
          const internal = isInternal(d.name);
          const selected = value === d.name;
          const running = info ? info.running : d.state.toLowerCase() === "running";
          return (
            <div
              key={d.name}
              className={cn(
                "flex items-center gap-3 rounded-md border bg-card px-3 py-2 text-sm",
                selected && "border-primary bg-accent/40",
                internal && "opacity-60",
              )}
            >
              <button
                type="button"
                role="radio"
                aria-checked={selected}
                disabled={internal}
                title={internal ? "Internal distribution of Docker Desktop or a Podman machine: use the named pipe connection instead" : undefined}
                onClick={() => onChange(d.name)}
                className="flex min-w-0 flex-1 flex-col items-start gap-0.5 text-left disabled:cursor-not-allowed"
              >
                <span className="flex items-center gap-2">
                  <span className="font-medium">{d.name}</span>
                  {d.default && <Badge variant="outline" className="rounded-md px-1.5 py-0 text-[11px]">Default</Badge>}
                  <Badge variant="outline" className="rounded-md px-1.5 py-0 text-[11px]">WSL {d.version}</Badge>
                  <Badge
                    variant="outline"
                    className={cn("rounded-md px-1.5 py-0 text-[11px]", running ? "border-success/30 text-success" : "text-muted-foreground")}
                  >
                    {running ? "Running" : d.state || "Stopped"}
                  </Badge>
                </span>
                <span className="text-xs">
                  <DockerLine d={d} info={info} loading={q?.isLoading ?? false} />
                  {d.version === 1 && <span className="ml-2 text-warning">WSL 1 cannot run Docker Engine reliably</span>}
                </span>
              </button>
              {!running && !internal && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => start.mutate(d.name)}
                  disabled={start.isPending}
                  title="Boots the distribution so Docker can be detected"
                >
                  {start.isPending && start.variables === d.name ? <Loader2 className="animate-spin" /> : <Play />}
                  Start WSL
                </Button>
              )}
            </div>
          );
        })}
        {value && list.length > 0 && !list.some((d) => d.name === value) && (
          <p className="text-xs text-warning">The saved distribution "{value}" is not installed.</p>
        )}
      </div>
      {value && selectedInfo?.running && !isInternal(value) && (
        <WslDockerSetup distro={value} daemonReachable={selectedInfo.daemonReachable} successAction={successAction} />
      )}
      <p className="text-xs text-muted-foreground">
        Connects through <code className="font-mono">wsl.exe</code> to the Docker Engine inside the distribution. No TCP port is exposed.
        A stopped distribution is not an error; start it explicitly.
      </p>
    </div>
  );
}
