import { useEffect } from "react";
import { useQueries, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
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

/**
 * Saved connections, other than `activeId`, whose last connection attempt failed. Used to flag a broken
 * engine on the picker now that the sidebar no longer lists every connection. Reads the same cached
 * status as useConnectionStatus; it never connects anything.
 */
export function useFailingConnections(conns: readonly ConnectionProfile[] | null | undefined, activeId: string | null): ConnectionProfile[] {
  const others = (conns ?? []).filter((c) => c.id !== activeId);
  const results = useQueries({
    queries: others.map((c) => ({
      queryKey: statusKey(c.id),
      queryFn: () => ConnectionService.Status(c.id),
      retry: false,
      staleTime: 10_000,
    })),
  });
  return others.filter((_, i) => results[i]?.data?.state === ConnectionState.StateError);
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

/** Runs one connection attempt and keeps the cached status in step with it. */
export async function connectAndTrack(qc: QueryClient, id: string, connect: (id: string) => Promise<ConnectionStatus>): Promise<void> {
  const connecting = () => qc.setQueryData(statusKey(id), blankStatus(id, ConnectionState.StateConnecting));
  connecting();
  // A status read still under way describes the connection before this attempt. Left alone, its
  // "connected" answer would open the pages while the backend is reconnecting, and their first
  // requests would fail. Cancelling restores the status from before the read, so it is set again.
  await qc.cancelQueries({ queryKey: statusKey(id) });
  connecting();
  try {
    qc.setQueryData(statusKey(id), await connect(id));
  } catch (e) {
    qc.setQueryData(statusKey(id), blankStatus(id, ConnectionState.StateError, errorMessage(e)));
  }
}

/** Selects a connection and starts a connection attempt. */
export function useSelectConnection() {
  const qc = useQueryClient();
  const set = useUI((s) => s.setActiveConnection);
  return async (id: string) => {
    set(id);
    await connectAndTrack(qc, id, (c) => ConnectionService.Connect(c));
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
