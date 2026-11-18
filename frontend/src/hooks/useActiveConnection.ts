import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { ConnectionService } from "@bindings/services";
import { ConnectionState, type ConnectionProfile, type ConnectionStatus } from "@bindings/model";
import { useUI } from "@/stores/uiStore";
import { errorMessage } from "@/lib/errors";

export const connectionsKey = ["connections"] as const;
export const statusKey = (id: string) => ["connection-status", id] as const;

export function useConnections() {
  return useQuery({
    queryKey: connectionsKey,
    queryFn: () => ConnectionService.List(),
    retry: false,
  });
}

/** Status of one connection; connecting is an explicit action (see useSelectConnection), never implicit. */
export function useConnectionStatus(id: string | null) {
  return useQuery({
    queryKey: statusKey(id ?? "none"),
    queryFn: () => ConnectionService.Status(id as string),
    enabled: !!id,
    retry: false,
    staleTime: 10_000,
  });
}

export function useActiveConnection(): {
  connectionId: string | null;
  profile?: ConnectionProfile;
  status?: ConnectionStatus;
  isConnected: boolean;
} {
  const id = useUI((s) => s.activeConnectionId);
  const { data: profiles } = useConnections();
  const { data: status } = useConnectionStatus(id);
  // Only trust a status that belongs to the active connection.
  const safe = status && status.connectionId === id ? status : undefined;
  return {
    connectionId: id,
    profile: profiles?.find((p) => p.id === id),
    status: safe,
    isConnected: safe?.state === ConnectionState.StateConnected,
  };
}

function blankStatus(id: string, state: ConnectionState, error?: string): ConnectionStatus {
  return {
    connectionId: id,
    state,
    error,
    containers: 0,
    images: 0,
    volumes: 0,
    networks: 0,
    capabilities: {
      canListResources: false,
      canExecuteContainers: false,
      canStreamLogs: false,
      canStreamStats: false,
      canRunCompose: false,
      canBuildImages: false,
    },
  } as ConnectionStatus;
}

/** Selects a connection and starts a connection attempt. */
export function useSelectConnection() {
  const qc = useQueryClient();
  const set = useUI((s) => s.setActiveConnection);
  return async (id: string) => {
    set(id);
    qc.setQueryData(statusKey(id), blankStatus(id, ConnectionState.StateConnecting));
    try {
      qc.setQueryData(statusKey(id), await ConnectionService.Connect(id));
    } catch (e) {
      qc.setQueryData(statusKey(id), blankStatus(id, ConnectionState.StateError, errorMessage(e)));
    }
  };
}

/** Subscribes to backend status pushes and writes them to the query cache. */
export function useConnectionStatusEvents() {
  const qc = useQueryClient();
  useEffect(() => {
    const off = Events.On("connection:status", (ev: { data: ConnectionStatus }) => {
      const st = ev.data;
      if (st?.connectionId) qc.setQueryData(statusKey(st.connectionId), st);
    });
    return () => off();
  }, [qc]);
}
