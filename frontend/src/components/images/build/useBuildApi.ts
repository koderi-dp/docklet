import { useEffect, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ImageService } from "@bindings/services";
import type { BuildPlan, BuildProgress, BuildRecord } from "@bindings/model";
import { useOnOperationDone } from "@/hooks/useOperations";
import type { BuildForm } from "./buildForm";
import { toSpec } from "./buildForm";

/** Value that follows `value` after it stayed unchanged for `ms` milliseconds. */
function useSettled<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

/**
 * Which builder a build with this folder would use, and whether the folder is reachable by it.
 * Only the fields that change the answer (folder, Dockerfile, builder choice) are sent.
 */
export function useBuildPlan(connectionId: string, form: Pick<BuildForm, "contextDir" | "dockerfile" | "builder">, enabled: boolean) {
  const key = JSON.stringify([form.contextDir.trim(), form.dockerfile.trim(), form.builder]);
  const settled = useSettled(key, 350);
  const [contextDir, dockerfile, builder] = JSON.parse(settled) as [string, string, BuildForm["builder"]];
  const q = useQuery({
    queryKey: ["build-plan", connectionId, contextDir, dockerfile, builder] as const,
    queryFn: (): Promise<BuildPlan> =>
      ImageService.BuildPlan(connectionId, { ...toSpec({ contextDir, dockerfile, builder, tags: [], args: [], target: "", platform: "", noCache: false, pull: false }) }),
    enabled,
    retry: false,
    staleTime: 15_000,
    placeholderData: keepPreviousData,
  });
  return { ...q, pending: key !== settled || q.isFetching };
}

export const buildHistoryKey = (connectionId: string) => ["build-history", connectionId] as const;

/** Builds started from Docklet on this computer for one connection, newest first. Refreshes when a build finishes. */
export function useBuildHistory(connectionId: string) {
  const qc = useQueryClient();
  useOnOperationDone("image.build", connectionId, () => {
    // The record is written just before the operation ends; one extra refetch covers any lag.
    void qc.invalidateQueries({ queryKey: buildHistoryKey(connectionId) });
    setTimeout(() => void qc.invalidateQueries({ queryKey: buildHistoryKey(connectionId) }), 800);
  });
  return useQuery({
    queryKey: buildHistoryKey(connectionId),
    queryFn: async (): Promise<BuildRecord[]> => (await ImageService.BuildHistory(connectionId)) ?? [],
    retry: false,
  });
}

export function useRemoveBuild(connectionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => ImageService.RemoveBuild(connectionId, id),
    onSettled: () => void qc.invalidateQueries({ queryKey: buildHistoryKey(connectionId) }),
  });
}

export function useClearBuilds(connectionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => ImageService.ClearBuildHistory(connectionId),
    onSettled: () => void qc.invalidateQueries({ queryKey: buildHistoryKey(connectionId) }),
  });
}

/**
 * Live step list of a build operation, polled while it runs. Resolves to an error for builds that have no
 * steps (Classic builder, or an operation from a previous session).
 */
export function useBuildProgress(operationId: string, enabled: boolean) {
  return useQuery({
    queryKey: ["build-progress", operationId] as const,
    queryFn: (): Promise<BuildProgress> => ImageService.BuildProgress(operationId),
    enabled,
    retry: false,
    refetchInterval: (q) => (q.state.status === "error" || q.state.data?.running === false ? false : 700),
  });
}
