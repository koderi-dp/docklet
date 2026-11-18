import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ConnectionService } from "@bindings/services";
import { ConnectionState } from "@bindings/model";
import { connectionsKey, statusKey, useSelectConnection } from "@/hooks/useActiveConnection";
import { useUI } from "@/stores/uiStore";
import { toastError } from "@/lib/errors";

/** Connect / disconnect / duplicate / remove with consistent cache updates and toasts. */
export function useConnectionActions() {
  const qc = useQueryClient();
  const select = useSelectConnection();
  const setActive = useUI((s) => s.setActiveConnection);
  const activeId = useUI((s) => s.activeConnectionId);

  const disconnect = async (id: string, name?: string) => {
    try {
      await ConnectionService.Disconnect(id);
      const prev = qc.getQueryData<Record<string, unknown>>(statusKey(id));
      qc.setQueryData(statusKey(id), { ...(prev ?? {}), connectionId: id, state: ConnectionState.StateDisconnected });
      toast.success(`Disconnected${name ? ` from ${name}` : ""}`);
    } catch (e) {
      toastError("Could not disconnect", e);
    }
  };

  const duplicate = async (id: string) => {
    try {
      const p = await ConnectionService.Duplicate(id);
      await qc.invalidateQueries({ queryKey: connectionsKey });
      toast.success(`Created "${p.name}"`);
      return p;
    } catch (e) {
      toastError("Could not duplicate the connection", e);
    }
  };

  const remove = async (id: string, name?: string) => {
    try {
      await ConnectionService.Remove(id);
      if (activeId === id) setActive(null);
      qc.removeQueries({ queryKey: statusKey(id) });
      await qc.invalidateQueries({ queryKey: connectionsKey });
      toast.success(`Removed ${name ?? "connection"}`);
      return true;
    } catch (e) {
      toastError("Could not remove the connection", e);
      return false;
    }
  };

  return { connect: select, disconnect, duplicate, remove };
}
