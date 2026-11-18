import { useCallback } from "react";
import { useQueries } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { HostService } from "@bindings/services";
import { useConnections } from "@/hooks/useActiveConnection";
import { toastError } from "@/lib/errors";
import { engineMayBeLocal } from "@/lib/hostPath";
import { useUI } from "@/stores/uiStore";

/**
 * Where engine paths are on this computer. An entry is `exists` only for a path that is on THIS
 * machine and present; remote engines never ask the backend.
 */
export function useLocalPaths(connectionId: string | null | undefined, enginePaths: readonly string[]) {
  const { data: profiles } = useConnections();
  const type = profiles?.find((p) => p.id === connectionId)?.type;
  const mayBeLocal = !!connectionId && engineMayBeLocal(type);
  return useQueries({
    queries: enginePaths.map((enginePath) => ({
      queryKey: ["host-path", connectionId, enginePath],
      queryFn: () => HostService.ResolveLocalPath(connectionId as string, enginePath),
      enabled: mayBeLocal && !!enginePath,
      staleTime: 30_000,
      retry: false,
    })),
    combine: (rs) => rs.map((r) => ({ exists: mayBeLocal && !!r.data?.exists, isDir: !!r.data?.isDir })),
  });
}

export function useLocalPath(connectionId: string | null | undefined, enginePath: string | undefined) {
  return useLocalPaths(connectionId, [enginePath ?? ""])[0];
}

/** Show folder / Open in editor for engine paths of one connection, with the toasts for the outcome. */
export function useHostActions(connectionId: string | null | undefined) {
  const navigate = useNavigate();
  const editor = useUI((s) => s.editor);
  const showFolder = useCallback(
    async (enginePath: string) => {
      if (!connectionId) return;
      try {
        await HostService.ShowInFileManager(connectionId, enginePath);
      } catch (e) {
        toastError("Could not show the folder", e);
      }
    },
    [connectionId],
  );
  const openInEditor = useCallback(
    async (enginePath: string) => {
      if (!connectionId) return;
      try {
        const r = await HostService.OpenInEditor(connectionId, enginePath, editor);
        if (r.needsSetting) {
          toast.error("No editor found", {
            description: r.message,
            duration: Infinity,
            closeButton: true,
            action: { label: "Open settings", onClick: () => navigate("/settings") },
          });
        }
      } catch (e) {
        toastError("Could not open the editor", e);
      }
    },
    [connectionId, editor, navigate],
  );
  return { showFolder, openInEditor };
}
