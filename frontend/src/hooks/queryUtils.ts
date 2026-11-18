import { useEffect, useRef, useSyncExternalStore } from "react";
import { useQueryClient, type InvalidateQueryFilters, type QueryClient } from "@tanstack/react-query";
import { Events } from "@wailsio/runtime";

/** Shared TanStack Query timings (the QueryClient in App.tsx uses the same defaults). */
export const STALE_MS = 5_000;
export const GC_MS = 5 * 60_000;

// ---- page visibility (one listener for the whole app) ----

const visibilityListeners = new Set<() => void>();
let visibilityBound = false;

function subscribeVisibility(cb: () => void) {
  if (!visibilityBound) {
    visibilityBound = true;
    document.addEventListener("visibilitychange", () => visibilityListeners.forEach((l) => l()));
  }
  visibilityListeners.add(cb);
  return () => {
    visibilityListeners.delete(cb);
  };
}

const isVisible = () => document.visibilityState !== "hidden";

/** True while the window is visible. Poll/stream only while this is true. */
export function usePageVisible(): boolean {
  return useSyncExternalStore(subscribeVisibility, isVisible, () => true);
}

// ---- keep previous rows only within the same connection ----

/**
 * placeholderData for queries whose key has extra parts after the connection id
 * (path, tail, filters): keeps the previous rows while the new key loads so tables never
 * blank, but never across connections (another engine's rows must not show up).
 */
export function keepForConnection<T>(connectionId: string | null) {
  return (prev: T | undefined, prevQuery?: { queryKey: readonly unknown[] }): T | undefined =>
    prevQuery && prevQuery.queryKey.includes(connectionId) ? prev : undefined;
}

// ---- single Wails subscription per event name, fan-out by connection ----

type EventListener = (connectionId: string | undefined) => void;
const hub = new Map<string, { off: () => void; listeners: Set<EventListener> }>();

export function payloadOf<T>(ev: { data?: unknown }): T | undefined {
  const raw = Array.isArray(ev.data) ? ev.data[0] : ev.data;
  return raw as T | undefined;
}

/** Subscribes to a backend event through ONE shared Events.On per name. Returns the unsubscribe function. */
export function subscribeBackendEvent(name: string, fn: EventListener): () => void {
  let h = hub.get(name);
  if (!h) {
    const listeners = new Set<EventListener>();
    const off = Events.On(name, (ev: { data?: unknown }) => {
      const conn = payloadOf<{ connectionId?: string }>(ev)?.connectionId;
      listeners.forEach((l) => l(conn));
    });
    h = { off, listeners };
    hub.set(name, h);
  }
  h.listeners.add(fn);
  const entry = h;
  return () => {
    entry.listeners.delete(fn);
    if (entry.listeners.size === 0) {
      entry.off();
      hub.delete(name);
    }
  };
}

// ---- coalesced invalidation: one invalidation per key per burst ----

const COALESCE_MS = 120;
const pending = new Map<string, { qc: QueryClient; filters: InvalidateQueryFilters }>();
let flushTimer: ReturnType<typeof setTimeout> | null = null;

/**
 * Schedules an invalidation; calls with the same `id` within COALESCE_MS (events bursts, several
 * components using the same hook) collapse into one.
 */
export function queueInvalidate(qc: QueryClient, id: string, filters: InvalidateQueryFilters) {
  pending.set(id, { qc, filters });
  if (flushTimer) return;
  flushTimer = setTimeout(() => {
    flushTimer = null;
    const batch = [...pending.values()];
    pending.clear();
    for (const { qc: client, filters: f } of batch) void client.invalidateQueries(f);
  }, COALESCE_MS);
}

/** Convenience: queue invalidation of a plain query key prefix. */
export function queueInvalidateKey(qc: QueryClient, queryKey: readonly unknown[]) {
  queueInvalidate(qc, JSON.stringify(queryKey), { queryKey });
}

/**
 * Runs cb (coalesced) when any of the named Engine events fires for THIS connection
 * (events of other engines are ignored).
 */
export function useConnectionEvents(names: string[], connectionId: string | null, enabled: boolean, cb: () => void) {
  const ref = useRef(cb);
  ref.current = cb;
  const key = names.join("|");
  useEffect(() => {
    if (!connectionId || !enabled) return;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const offs = key.split("|").map((n) =>
      subscribeBackendEvent(n, (conn) => {
        if (conn && conn !== connectionId) return;
        if (timer) return; // burst: the pending call covers it
        timer = setTimeout(() => {
          timer = null;
          ref.current();
        }, COALESCE_MS);
      }),
    );
    return () => {
      offs.forEach((off) => off());
      if (timer) clearTimeout(timer);
    };
  }, [key, connectionId, enabled]);
}

export type InvalidateTarget = readonly unknown[] | { id: string; filters: InvalidateQueryFilters };

/**
 * Invalidates the given query keys (coalesced and de-duplicated app-wide) when any of the named
 * events fires for THIS connection. Subscriptions are shared, so many components can use it freely.
 */
export function useInvalidateOnEvents(names: string[], connectionId: string | null, enabled: boolean, targets: InvalidateTarget[]) {
  const qc = useQueryClient();
  const ref = useRef(targets);
  ref.current = targets;
  const key = names.join("|");
  useEffect(() => {
    if (!connectionId || !enabled) return;
    const offs = key.split("|").map((n) =>
      subscribeBackendEvent(n, (conn) => {
        if (conn && conn !== connectionId) return;
        for (const t of ref.current) {
          if (Array.isArray(t)) queueInvalidateKey(qc, t);
          else {
            const o = t as { id: string; filters: InvalidateQueryFilters };
            queueInvalidate(qc, o.id, o.filters);
          }
        }
      }),
    );
    return () => offs.forEach((off) => off());
  }, [qc, key, connectionId, enabled]);
}
