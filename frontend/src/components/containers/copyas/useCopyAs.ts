import { useQuery } from "@tanstack/react-query";
import { CopyAsService } from "@bindings/services";
import type { CopyAsShell } from "@/stores/uiStore";

/** Which "Copy as" entries apply (database detection) and which variables are secret-looking. */
export function useCopyAsInfo(connectionId: string | null, containerId: string) {
  return useQuery({
    queryKey: ["copyas-info", connectionId, containerId],
    queryFn: () => CopyAsService.Info(connectionId as string, containerId),
    enabled: !!connectionId,
    staleTime: 15_000,
    retry: false,
  });
}

/** The default text (secret values replaced by variable references). Never cached across opens. */
export function useCopyAsText(kind: "run" | "compose", connectionId: string, containerId: string, shell: CopyAsShell) {
  return useQuery({
    queryKey: ["copyas-text", kind, connectionId, containerId, kind === "run" ? shell : ""],
    queryFn: () =>
      kind === "run"
        ? CopyAsService.DockerRun(connectionId, containerId, shell, false)
        : CopyAsService.Compose(connectionId, containerId, false),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
}

export function useConnectionStrings(connectionId: string, containerId: string) {
  return useQuery({
    queryKey: ["copyas-conn", connectionId, containerId],
    queryFn: () => CopyAsService.ConnectionStrings(connectionId, containerId, false),
    gcTime: 0,
    staleTime: 0,
    retry: false,
  });
}
