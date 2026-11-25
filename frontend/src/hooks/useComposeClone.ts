import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ComposeService } from "@bindings/services";
import { OperationStatus } from "@bindings/model";
import { awaitOperation } from "@/lib/awaitOperation";

/** Renames a saved project without containers: only Docklet's saved entry changes. */
export function useRenameComposeProject(connectionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { name: string; newName: string }) => ComposeService.RenameProject(connectionId, v.name, v.newName),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["compose", connectionId] }),
  });
}

/** What cloning the project would copy and what the copy would share with the original. */
export function useClonePreview(connectionId: string, name: string, enabled: boolean) {
  return useQuery({
    queryKey: ["compose", connectionId, "clone-preview", name],
    queryFn: () => ComposeService.ClonePreview(connectionId, name),
    enabled,
    staleTime: 0,
    gcTime: 0,
    retry: false,
  });
}

/**
 * Copies the project folder and saves the copy under a new name. Resolves when the copy is
 * finished and rejects with the operation's reason when it failed or was cancelled.
 */
export function useCloneComposeProject(connectionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { name: string; newName: string; dir: string }) => {
      const started = await ComposeService.CloneProject(connectionId, v.name, v.newName, v.dir);
      const done = await awaitOperation(started.id);
      if (done.status !== OperationStatus.OpSucceeded) {
        throw new Error(done.status === OperationStatus.OpFailed ? done.error || "The copy failed" : "The copy was cancelled; nothing was kept.");
      }
      return v.newName;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["compose", connectionId] }),
  });
}
