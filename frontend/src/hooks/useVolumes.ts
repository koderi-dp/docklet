import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { VolumeService } from "@bindings/services";
import { useOnOperationDone } from "@/hooks/useOperations";
import { queueInvalidateKey, useConnectionEvents, useInvalidateOnEvents } from "@/hooks/queryUtils";

export { useConnectionEvents };

/** All query keys include connectionId so engines never share cache entries. */
export const volumesKey = (connectionId: string) => ["volumes", connectionId] as const;

const LIST_FALLBACK_MS = 30_000;

export function useVolumesQuery(connectionId: string | null, enabled = true) {
  const qc = useQueryClient();
  const refresh = () => {
    if (connectionId) queueInvalidateKey(qc, volumesKey(connectionId));
  };
  // containers:changed because "in use" is derived from container mounts.
  useInvalidateOnEvents(["volumes:changed", "containers:changed"], connectionId, enabled, [volumesKey(connectionId ?? "none")]);
  useOnOperationDone("volume.", connectionId, refresh);
  useOnOperationDone("system.", connectionId, refresh);
  return useQuery({
    queryKey: volumesKey(connectionId ?? "none"),
    queryFn: () => VolumeService.List(connectionId as string),
    enabled: !!connectionId && enabled,
    refetchInterval: LIST_FALLBACK_MS,
    retry: false,
  });
}

export function useRemoveVolumes(connectionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (names: string[]) => VolumeService.Remove(connectionId, names, false),
    onSettled: () => void qc.invalidateQueries({ queryKey: volumesKey(connectionId) }),
  });
}

export function useCreateVolume(connectionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { name: string; driver: string; labels: Record<string, string> }) =>
      VolumeService.Create(connectionId, v.name, v.driver, v.labels),
    onSuccess: () => void qc.invalidateQueries({ queryKey: volumesKey(connectionId) }),
  });
}

export function usePruneVolumes(connectionId: string) {
  return useMutation({
    // The dialog collects the explicit confirmation before this is ever called.
    mutationFn: () => VolumeService.PruneUnused(connectionId, true),
  });
}
