import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { WSLService } from "@bindings/services";
import { OperationStatus } from "@bindings/model";
import { useOnOperationDone } from "@/hooks/useOperations";
import { queueInvalidateKey, usePageVisible } from "@/hooks/queryUtils";
import { toastError } from "@/lib/errors";

export const wslInfoKey = (connectionId: string) => ["wsl-info", connectionId] as const;

/**
 * State of the WSL distribution behind a connection. Refreshed every 10 s, and only while the window is
 * visible; the tab unmounts when it is left, so nothing polls in the background.
 */
export function useWslInfo(connectionId: string | null) {
  const visible = usePageVisible();
  return useQuery({
    queryKey: wslInfoKey(connectionId ?? "none"),
    queryFn: () => WSLService.Info(connectionId as string),
    enabled: !!connectionId,
    refetchInterval: visible ? 10_000 : false,
    staleTime: 5_000,
    retry: false,
  });
}

/** Starts a maintenance action (an operation with live output in Operations) and refreshes afterwards. */
export function useWslActions(connectionId: string) {
  const qc = useQueryClient();
  const refresh = () => queueInvalidateKey(qc, wslInfoKey(connectionId));
  useOnOperationDone("wsl.", connectionId, (op) => {
    refresh();
    if (op.status === OperationStatus.OpSucceeded) toast.success(`${op.title} finished`, { description: op.detail });
    else if (op.status === OperationStatus.OpFailed)
      toast.error(`${op.title} failed`, { description: op.error, duration: Infinity, closeButton: true });
  });
  return useMutation({
    mutationFn: (action: string) => WSLService.Run(connectionId, action),
    onSuccess: (op) => toast.info(`${op.title}: started`, { description: "Live output is in Operations." }),
    onError: (e) => toastError("Could not start the action", e),
    onSettled: refresh,
  });
}
