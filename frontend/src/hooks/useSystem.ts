import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { SystemService } from "@bindings/services";
import { useOnOperationDone } from "@/hooks/useOperations";
import { keepForConnection, queueInvalidateKey, usePageVisible } from "@/hooks/queryUtils";

export const engineInfoKey = (connectionId: string) => ["engine-info", connectionId] as const;
export const diskUsageKey = (connectionId: string) => ["disk-usage", connectionId] as const;

export function useEngineInfo(connectionId: string | null) {
  return useQuery({
    queryKey: engineInfoKey(connectionId ?? "none"),
    queryFn: () => SystemService.Info(connectionId as string),
    enabled: !!connectionId,
    staleTime: 15_000,
    retry: false,
  });
}

/** Docker disk accounting (docker system df); refetched when a prune finishes. */
export function useDiskUsage(connectionId: string | null, enabled = true) {
  const qc = useQueryClient();
  const refresh = () => {
    if (connectionId) queueInvalidateKey(qc, diskUsageKey(connectionId));
  };
  useOnOperationDone("system.", connectionId, refresh);
  useOnOperationDone("volume.", connectionId, refresh);
  useOnOperationDone("network.", connectionId, refresh);
  return useQuery({
    queryKey: diskUsageKey(connectionId ?? "none"),
    queryFn: () => SystemService.DiskUsage(connectionId as string),
    enabled: !!connectionId && enabled,
    staleTime: 15_000,
    retry: false,
  });
}

/** One-shot recent events; polls only while autoRefresh is on, the page is visible and enabled. */
export function useRecentEvents(
  connectionId: string | null,
  sinceSeconds: number,
  limit: number,
  autoRefresh: boolean,
  enabled = true,
) {
  const visible = usePageVisible();
  return useQuery({
    queryKey: ["engine-events", connectionId ?? "none", sinceSeconds, limit] as const,
    queryFn: () => SystemService.RecentEvents(connectionId as string, sinceSeconds, limit),
    enabled: !!connectionId && enabled,
    refetchInterval: autoRefresh && visible ? 10_000 : false,
    placeholderData: keepForConnection(connectionId),
    retry: false,
  });
}

export type PruneKind =
  | "containers"
  | "images"
  | "images-all"
  | "networks"
  | "volumes"
  | "buildcache"
  | "system"
  | "system-volumes";

/** Starts the prune operation; destructive flags are set only by the confirmation dialog's explicit kinds. */
export function usePrune(connectionId: string) {
  return useMutation({
    mutationFn: (kind: PruneKind) => {
      switch (kind) {
        case "containers":
          return SystemService.PruneContainers(connectionId);
        case "images":
          return SystemService.PruneImages(connectionId, false);
        case "images-all":
          return SystemService.PruneImages(connectionId, true);
        case "networks":
          return SystemService.PruneNetworks(connectionId);
        case "volumes":
          return SystemService.PruneVolumes(connectionId, true);
        case "buildcache":
          return SystemService.PruneBuildCache(connectionId, false);
        case "system":
          return SystemService.SystemPrune(connectionId, false, false, false);
        case "system-volumes":
          return SystemService.SystemPrune(connectionId, false, true, true);
      }
    },
  });
}
