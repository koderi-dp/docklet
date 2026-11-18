import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { NetworkService } from "@bindings/services";
import type { NetworkCreateRequest } from "@bindings/model";
import { useOnOperationDone } from "@/hooks/useOperations";
import { queueInvalidateKey, useInvalidateOnEvents } from "@/hooks/queryUtils";

export const networksKey = (connectionId: string) => ["networks", connectionId] as const;
export const networkDetailKey = (connectionId: string, id: string) => ["network-detail", connectionId, id] as const;

const LIST_FALLBACK_MS = 30_000;

export function useNetworksQuery(connectionId: string | null, enabled = true) {
  const qc = useQueryClient();
  const refresh = () => {
    if (!connectionId) return;
    queueInvalidateKey(qc, networksKey(connectionId));
    queueInvalidateKey(qc, ["network-detail", connectionId]);
  };
  useInvalidateOnEvents(["networks:changed", "containers:changed"], connectionId, enabled, [
    networksKey(connectionId ?? "none"),
    ["network-detail", connectionId ?? "none"],
  ]);
  useOnOperationDone("network.", connectionId, refresh);
  useOnOperationDone("system.", connectionId, refresh);
  return useQuery({
    queryKey: networksKey(connectionId ?? "none"),
    queryFn: () => NetworkService.List(connectionId as string),
    enabled: !!connectionId && enabled,
    refetchInterval: LIST_FALLBACK_MS,
    retry: false,
  });
}

/** Engine inspect: authoritative IPAM, options and endpoint addresses. */
export function useNetworkDetail(connectionId: string, id: string | null) {
  return useQuery({
    queryKey: networkDetailKey(connectionId, id ?? "none"),
    queryFn: () => NetworkService.Inspect(connectionId, id as string),
    enabled: !!id,
    retry: false,
  });
}

function useInvalidate(connectionId: string) {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: networksKey(connectionId) });
    void qc.invalidateQueries({ queryKey: ["network-detail", connectionId] });
  };
}

export function useRemoveNetworks(connectionId: string) {
  const inv = useInvalidate(connectionId);
  return useMutation({
    mutationFn: (ids: string[]) => NetworkService.Remove(connectionId, ids),
    onSettled: inv,
  });
}

export function useCreateNetwork(connectionId: string) {
  const inv = useInvalidate(connectionId);
  return useMutation({
    mutationFn: (req: NetworkCreateRequest) => NetworkService.Create(connectionId, req),
    onSuccess: inv,
  });
}

export function useConnectContainer(connectionId: string) {
  const inv = useInvalidate(connectionId);
  return useMutation({
    mutationFn: (v: { networkId: string; containerId: string; connect: boolean }) =>
      v.connect
        ? NetworkService.Connect(connectionId, v.networkId, v.containerId)
        : NetworkService.Disconnect(connectionId, v.networkId, v.containerId),
    onSettled: inv,
  });
}

export function usePruneNetworks(connectionId: string) {
  return useMutation({ mutationFn: () => NetworkService.PruneUnused(connectionId) });
}
