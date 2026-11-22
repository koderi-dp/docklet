import { useEffect, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ComposeService } from "@bindings/services";
import { savedContent } from "@/lib/composeFileState";
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

export const composeApplyPlanKey = (c: string, n: string, profiles: readonly string[]) => ["compose", c, "applyplan", n, profiles.join(",")] as const;

/** What `compose up` would change. Always refetched when used; also on container events (live segment). */
export function useApplyPlan(connectionId: string | null, name: string, profiles: string[], enabled: boolean) {
  return useQuery({
    queryKey: composeApplyPlanKey(connectionId ?? "none", name, profiles),
    queryFn: () => ComposeService.ApplyPlan(connectionId as string, name, profiles),
    enabled: !!connectionId && !!name && enabled,
    staleTime: 0,
    retry: false,
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
  const query = useQuery({
    queryKey: ["compose", "outline", settled],
    queryFn: () => ComposeService.FileOutline(settled),
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
    gcTime: 60_000,
    retry: false,
  });
  // The kept data of an earlier text is a placeholder: it belongs to no text we can name.
  return { data: query.data, dataText: query.isPlaceholderData ? null : settled };
}

/**
 * After a save: the file's own cache is set from the result (a refetch would hand the editor a new
 * object for text it already shows), and everything else the file may have changed is marked stale.
 */
export function useInvalidateAfterSave(connectionId: string, name: string) {
  const qc = useQueryClient();
  return (path: string, sentText: string, version: string) => {
    qc.setQueryData(composeFileKey(connectionId, name, path), savedContent(path, sentText, version));
    for (const queryKey of [
      composeProjectKey(connectionId, name),
      composeConfigKey(connectionId, name),
      composeGraphKey(connectionId, name),
      ["compose", connectionId, "applyplan", name],
    ])
      void qc.invalidateQueries({ queryKey });
  };
}
