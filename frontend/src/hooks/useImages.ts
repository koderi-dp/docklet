import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ImageService } from "@bindings/services";
import type { ActionResult, BuildSpec, Operation } from "@bindings/model";
import { useOnOperationDone } from "@/hooks/useOperations";
import { queueInvalidateKey, useInvalidateOnEvents } from "@/hooks/queryUtils";

/** All query keys include connectionId so engines never share cache entries. */
export const imagesKey = (connectionId: string) => ["images", connectionId] as const;
export const imageDetailKey = (connectionId: string, ref: string) => ["image-detail", connectionId, ref] as const;
export const imageHistoryKey = (connectionId: string, ref: string) => ["image-history", connectionId, ref] as const;

const LIST_FALLBACK_MS = 60_000;

export function useImagesQuery(connectionId: string | null, enabled: boolean) {
  const qc = useQueryClient();

  // Containers starting/stopping/removing change the in-use flag.
  useInvalidateOnEvents(["images:changed", "containers:changed"], connectionId, enabled, [
    imagesKey(connectionId ?? "none"),
    ["image-detail", connectionId ?? "none"],
    ["image-history", connectionId ?? "none"],
  ]);

  useOnOperationDone("image.", connectionId, () => {
    if (connectionId) queueInvalidateKey(qc, imagesKey(connectionId));
  });

  return useQuery({
    queryKey: imagesKey(connectionId ?? "none"),
    queryFn: () => ImageService.List(connectionId as string),
    enabled: !!connectionId && enabled,
    refetchInterval: LIST_FALLBACK_MS,
    retry: false,
  });
}

export function useImageDetail(connectionId: string | null, ref: string | null) {
  return useQuery({
    queryKey: imageDetailKey(connectionId ?? "none", ref ?? "none"),
    queryFn: () => ImageService.Inspect(connectionId as string, ref as string),
    enabled: !!connectionId && !!ref,
    retry: false,
  });
}

export function useImageHistory(connectionId: string | null, ref: string | null) {
  return useQuery({
    queryKey: imageHistoryKey(connectionId ?? "none", ref ?? "none"),
    queryFn: () => ImageService.History(connectionId as string, ref as string),
    enabled: !!connectionId && !!ref,
    retry: false,
  });
}

export function useRemoveImages(connectionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { refs: string[]; force: boolean }): Promise<ActionResult[]> =>
      (await ImageService.Remove(connectionId, v.refs, v.force)) ?? [],
    onSettled: () => void qc.invalidateQueries({ queryKey: imagesKey(connectionId) }),
  });
}

export function useTagImage(connectionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { source: string; target: string }) => ImageService.Tag(connectionId, v.source, v.target),
    onSettled: () => void qc.invalidateQueries({ queryKey: imagesKey(connectionId) }),
  });
}

export function usePullImage(connectionId: string) {
  return useMutation({
    mutationFn: (ref: string): Promise<Operation> => ImageService.Pull(connectionId, ref),
  });
}

export function usePruneDangling(connectionId: string) {
  return useMutation({
    mutationFn: (): Promise<Operation> => ImageService.PruneDangling(connectionId),
  });
}

export function useRegistryAuthStatus(connectionId: string, ref: string | null) {
  return useQuery({
    queryKey: ["registry-auth", connectionId, ref ?? "none"] as const,
    queryFn: () => ImageService.RegistryAuthStatus(connectionId, ref as string),
    enabled: !!ref,
    retry: false,
    staleTime: 15_000,
  });
}

export function usePushImage(connectionId: string) {
  return useMutation({
    mutationFn: (ref: string): Promise<Operation> => ImageService.Push(connectionId, ref),
  });
}

export function useBuildImage(connectionId: string) {
  return useMutation({
    mutationFn: (spec: BuildSpec): Promise<Operation> => ImageService.Build(connectionId, spec),
  });
}

export function useSaveImages(connectionId: string) {
  return useMutation({
    mutationFn: (v: { refs: string[]; destPath: string }): Promise<Operation> => ImageService.Save(connectionId, v.refs, v.destPath),
  });
}

export function useLoadImage(connectionId: string) {
  return useMutation({
    mutationFn: (srcPath: string): Promise<Operation> => ImageService.Load(connectionId, srcPath),
  });
}
