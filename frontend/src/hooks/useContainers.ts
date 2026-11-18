import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";
import { ContainerService } from "@bindings/services";
import type { ActionResult, ContainerStat, ContainerStatsData } from "@bindings/model";
import type { ContainerAction } from "@/lib/actionEligibility";
import { queueInvalidateKey, useInvalidateOnEvents, usePageVisible } from "@/hooks/queryUtils";

/** All query keys include connectionId so engines never share cache entries. */
export const containersKey = (connectionId: string) => ["containers", connectionId] as const;
export const detailKey = (connectionId: string, id: string) => ["container-detail", connectionId, id] as const;

/** Low-frequency fallback; the `containers:changed` event drives timely refreshes. */
const LIST_FALLBACK_MS = 30_000;
/** The backend drops a stats watch nobody touched for ~45s; re-send the ids well within that. */
const STATS_HEARTBEAT_MS = 15_000;

export function useContainersQuery(connectionId: string | null, enabled: boolean) {
  // Events trigger targeted, coalesced invalidation scoped to the connection named in the event.
  useInvalidateOnEvents(["containers:changed"], connectionId, enabled, [containersKey(connectionId ?? "none")]);

  return useQuery({
    queryKey: containersKey(connectionId ?? "none"),
    queryFn: () => ContainerService.List(connectionId as string),
    enabled: !!connectionId && enabled,
    refetchInterval: LIST_FALLBACK_MS,
    retry: false,
  });
}

// One app-wide subscription to the batched stats event; watches register by id.
const statsSinks = new Map<string, (d: ContainerStatsData) => void>();
let statsOff: (() => void) | null = null;

function addStatsSink(watchId: string, fn: (d: ContainerStatsData) => void) {
  if (!statsOff) {
    statsOff = Events.On("containerstats:data", (ev: { data?: unknown }) => {
      const raw = Array.isArray(ev.data) ? ev.data[0] : ev.data;
      const d = raw as ContainerStatsData | undefined;
      if (d?.watchId) statsSinks.get(d.watchId)?.(d);
    });
  }
  statsSinks.set(watchId, fn);
  return () => {
    statsSinks.delete(watchId);
    if (statsSinks.size === 0 && statsOff) {
      statsOff();
      statsOff = null;
    }
  };
}

/**
 * Live CPU/memory for the given RUNNING, visible container ids. Backed by one backend stats watch
 * (streaming, batched once per second) that exists only while the page is visible.
 * Returns id -> stat; missing / nil fields mean "not available" and render as an em dash.
 */
export function useContainerStats(connectionId: string | null, runningVisibleIds: string[], enabled: boolean) {
  const visible = usePageVisible();
  const ids = useMemo(() => [...runningVisibleIds].sort(), [runningVisibleIds]);
  const idsKey = ids.join(",");
  const idsRef = useRef(ids);
  idsRef.current = ids;
  const watchRef = useRef<string | null>(null);
  const [raw, setRaw] = useState<{ conn: string | null; map: Map<string, ContainerStat> }>({ conn: null, map: new Map() });
  const [error, setError] = useState<Error | null>(null);
  const hasIds = ids.length > 0;
  const active = !!connectionId && enabled && visible && hasIds;

  // The watch lives while the page is visible and there is something to watch.
  useEffect(() => {
    if (!connectionId || !enabled || !visible || !hasIds) return;
    let cancelled = false;
    let off: (() => void) | null = null;
    let beat: ReturnType<typeof setInterval> | null = null;
    setError(null);
    ContainerService.StartStatsWatch(connectionId, idsRef.current)
      .then((watchId) => {
        if (cancelled) {
          void ContainerService.StopStatsWatch(watchId).catch(() => undefined);
          return;
        }
        watchRef.current = watchId;
        off = addStatsSink(watchId, (d) => {
          if (d.connectionId !== connectionId) return;
          const wanted = new Set(idsRef.current);
          setRaw((prev) => {
            const map = new Map<string, ContainerStat>();
            if (prev.conn === connectionId) for (const [k, v] of prev.map) if (wanted.has(k)) map.set(k, v);
            for (const st of d.stats ?? []) if (wanted.has(st.id)) map.set(st.id, st);
            return { conn: connectionId, map };
          });
        });
        // The ids may have changed while Start was in flight.
        void ContainerService.UpdateStatsWatch(watchId, idsRef.current).catch(() => {});
        beat = setInterval(() => void ContainerService.UpdateStatsWatch(watchId, idsRef.current).catch(() => {}), STATS_HEARTBEAT_MS);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e : new Error(String(e)));
      });
    return () => {
      cancelled = true;
      if (beat) clearInterval(beat);
      off?.();
      const id = watchRef.current;
      watchRef.current = null;
      if (id) void ContainerService.StopStatsWatch(id).catch(() => {});
    };
  }, [connectionId, enabled, visible, hasIds]);

  // The visible running set changed: tell the backend (the other streams keep running).
  useEffect(() => {
    const id = watchRef.current;
    if (id && idsRef.current.length > 0) void ContainerService.UpdateStatsWatch(id, idsRef.current).catch(() => {});
  }, [idsKey]);

  const map = useMemo(() => {
    const m = new Map<string, ContainerStat>();
    if (!active || raw.conn !== connectionId) return m;
    const wanted = new Set(ids);
    for (const [k, v] of raw.map) if (wanted.has(k)) m.set(k, v);
    return m;
  }, [raw, active, connectionId, ids]);
  return { stats: map, error, isLoading: active && map.size === 0 && !error };
}

export function useContainerDetail(connectionId: string | null, id: string | null) {
  return useQuery({
    queryKey: detailKey(connectionId ?? "none", id ?? "none"),
    queryFn: () => ContainerService.Inspect(connectionId as string, id as string),
    enabled: !!connectionId && !!id,
    retry: false,
  });
}

export interface BatchOutcome {
  action: ContainerAction;
  results: ActionResult[];
  /** Containers filtered out as ineligible before the request. */
  skippedCount: number;
}

/** Engine batch lifecycle operation scoped to one connection. Resolves only after the Engine replied. */
export function useContainerAction(connectionId: string | null) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (v: { action: ContainerAction; ids: string[]; skippedCount: number; force?: boolean }): Promise<BatchOutcome> => {
      if (!connectionId) throw new Error("No active connection");
      let results: ActionResult[];
      switch (v.action) {
        case "start":
          results = (await ContainerService.Start(connectionId, v.ids)) ?? [];
          break;
        case "stop":
          results = (await ContainerService.Stop(connectionId, v.ids)) ?? [];
          break;
        case "restart":
          results = (await ContainerService.Restart(connectionId, v.ids)) ?? [];
          break;
        case "remove":
          results = (await ContainerService.Remove(connectionId, v.ids, !!v.force)) ?? [];
          break;
        case "pause":
          results = (await ContainerService.Pause(connectionId, v.ids)) ?? [];
          break;
        case "unpause":
          results = (await ContainerService.Unpause(connectionId, v.ids)) ?? [];
          break;
        case "kill":
          results = (await ContainerService.Kill(connectionId, v.ids)) ?? [];
          break;
      }
      return { action: v.action, results, skippedCount: v.skippedCount };
    },
    onSettled: () => {
      if (!connectionId) return;
      queueInvalidateKey(qc, containersKey(connectionId));
      queueInvalidateKey(qc, ["container-detail", connectionId]);
      queueInvalidateKey(qc, ["container-top", connectionId]);
      queueInvalidateKey(qc, ["container-inspect-raw", connectionId]);
    },
  });
}
