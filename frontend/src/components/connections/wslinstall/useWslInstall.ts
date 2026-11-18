import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ConnectionService } from "@bindings/services";
import { OperationStatus, type Operation } from "@bindings/model";
import { useOpsStore } from "@/hooks/useOperations";

export const INSTALL_KIND = "wsl.install-docker";

export const wslInfoKey = (distro: string) => ["wsl-docker", distro] as const;
export const wslPlanKey = (distro: string) => ["wsl-plan", distro] as const;

/** Read-only inspection of a distribution: what Docker looks like there and what an install would do. */
export function useWslPlan(distro: string, enabled = true) {
  return useQuery({
    queryKey: wslPlanKey(distro),
    queryFn: () => ConnectionService.WSLDockerInstallPlan(distro),
    enabled: enabled && !!distro,
    retry: false,
    staleTime: 0,
  });
}

/** Re-runs detection everywhere the distribution's Docker state is shown. */
export function useRefreshWslDocker() {
  const qc = useQueryClient();
  return (distro: string) =>
    Promise.all([
      qc.invalidateQueries({ queryKey: ["wsl-docker"] }),
      qc.invalidateQueries({ queryKey: ["wsl-distros"] }),
      qc.invalidateQueries({ queryKey: wslPlanKey(distro) }),
    ]);
}

/** The newest install operation for a distribution (running or finished), if any. */
export function useInstallOperation(distro: string, id?: string): Operation | undefined {
  return useOpsStore((s) => {
    if (id) return s.ops[id];
    return Object.values(s.ops)
      .filter((o) => o.kind === INSTALL_KIND && o.resource === distro && o.status === OperationStatus.OpRunning)
      .sort((a, b) => b.startedAt - a.startedAt)[0];
  });
}

const STEP_LINE = /^\[(\d+)\/(\d+)\] (.+)$/;

/** The step marker lines the backend logs as each step starts. */
export function currentStep(lines: string[]): { index: number; total: number; title: string } | null {
  for (let i = lines.length - 1; i >= 0; i--) {
    const m = STEP_LINE.exec(lines[i]);
    if (m) return { index: Number(m[1]), total: Number(m[2]), title: m[3] };
  }
  return null;
}
