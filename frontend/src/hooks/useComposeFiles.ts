import { useEffect, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ComposeService } from "@bindings/services";
import { composeConfigKey, composeGraphKey, composeProjectKey } from "@/hooks/useCompose";

export const composeFilesKey = (c: string, n: string) => ["compose", c, "files", n] as const;
export const composeFileKey = (c: string, n: string, path: string) => ["compose", c, "file", n, path] as const;

export function useComposeFiles(connectionId: string | null, name: string, enabled: boolean) {
  return useQuery({
    queryKey: composeFilesKey(connectionId ?? "none", name),
    queryFn: () => ComposeService.ProjectFiles(connectionId as string, name),
    enabled: !!connectionId && !!name && enabled,
    retry: false,
  });
}

/**
 * One file as the editor needs it. It is read again only on request: a refetch behind the user's
 * back must never replace text they are typing.
 */
export function useComposeFile(connectionId: string | null, name: string, path: string, enabled: boolean) {
  return useQuery({
    queryKey: composeFileKey(connectionId ?? "none", name, path),
    queryFn: () => ComposeService.ReadProjectFile(connectionId as string, name, path),
    enabled: !!connectionId && !!name && !!path && enabled,
    staleTime: Infinity,
    gcTime: 0,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    retry: false,
  });
}

export interface SaveVars {
  path: string;
  text: string;
  baseVersion: string;
  overwrite: boolean;
}

export function useSaveComposeFile(connectionId: string, name: string) {
  return useMutation({
    mutationFn: (v: SaveVars) => ComposeService.SaveProjectFile(connectionId, name, v.path, v.text, v.baseVersion, v.overwrite),
  });
}

const OUTLINE_DELAY_MS = 300;

/** The services of the text and the YAML error that stops it, 300 ms after the last keystroke. */
export function useFileOutline(text: string, enabled: boolean) {
  const [settled, setSettled] = useState(text);
  useEffect(() => {
    const t = setTimeout(() => setSettled(text), OUTLINE_DELAY_MS);
    return () => clearTimeout(t);
  }, [text]);
  return useQuery({
    queryKey: ["compose", "outline", settled],
    queryFn: () => ComposeService.FileOutline(settled),
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
    gcTime: 60_000,
    retry: false,
  });
}

/** Marks everything the saved file may have changed as stale. */
export function useInvalidateAfterSave(connectionId: string, name: string) {
  const qc = useQueryClient();
  return (path: string) => {
    for (const queryKey of [
      composeFileKey(connectionId, name, path),
      composeProjectKey(connectionId, name),
      composeConfigKey(connectionId, name),
      composeGraphKey(connectionId, name),
      ["compose", connectionId, "applyplan", name],
    ])
      void qc.invalidateQueries({ queryKey });
  };
}
