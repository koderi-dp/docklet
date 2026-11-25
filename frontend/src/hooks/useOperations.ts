import { useEffect, useMemo, useRef } from "react";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import { create } from "zustand";
import { Events } from "@wailsio/runtime";
import { toast } from "sonner";
import { OperationService } from "@bindings/services";
import { OperationStatus, type Operation, type OperationLog } from "@bindings/model";
import { errorMessage } from "@/lib/errors";
import { failureToastWanted, opToastId } from "@/lib/toastLayer";

/** Per-operation log lines kept client-side (the backend buffer is bigger; the view only needs the tail). */
export const MAX_LOG_LINES = 1000;

interface OpsState {
  ops: Record<string, Operation>;
  loaded: boolean;
  loadError: string | null;
  /** Logs of operations whose log view was opened (loaded from the backend, then appended from events). */
  logs: Record<string, string[]>;
  /** The Operations side panel, and the operation it was opened for or the user picked (null: let the panel choose). */
  panelOpen: boolean;
  selectedId: string | null;
  openPanel: (id?: string) => void;
  closePanel: () => void;
  select: (id: string) => void;
  upsert: (op: Operation) => void;
  appendLogs: (id: string, lines: string[]) => void;
  setLogs: (id: string, lines: string[]) => void;
  setAll: (ops: Operation[]) => void;
  remove: (ids: string[]) => void;
}

export const useOpsStore = create<OpsState>((set) => ({
  ops: {},
  loaded: false,
  loadError: null,
  logs: {},
  panelOpen: false,
  selectedId: null,
  openPanel: (id) => set((s) => ({ panelOpen: true, selectedId: id ?? s.selectedId })),
  closePanel: () => set({ panelOpen: false }),
  select: (selectedId) => set({ selectedId }),
  upsert: (op) => set((s) => ({ ops: { ...s.ops, [op.id]: op } })),
  appendLogs: (id, lines) =>
    set((s) => {
      const cur = s.logs[id];
      if (!cur) return s; // log view never opened: the backend buffer will be loaded on demand
      const next = cur.concat(lines);
      return { logs: { ...s.logs, [id]: next.length > MAX_LOG_LINES ? next.slice(next.length - MAX_LOG_LINES) : next } };
    }),
  setLogs: (id, lines) => set((s) => ({ logs: { ...s.logs, [id]: lines.slice(-MAX_LOG_LINES) } })),
  setAll: (list) =>
    set((s) => {
      const ops: Record<string, Operation> = {};
      for (const o of list) ops[o.id] = o;
      // Keep anything that arrived through events while the initial list was loading.
      for (const [id, o] of Object.entries(s.ops)) if (!ops[id]) ops[id] = o;
      return { ops, loaded: true, loadError: null };
    }),
  remove: (ids) =>
    set((s) => {
      const ops = { ...s.ops };
      const logs = { ...s.logs };
      for (const id of ids) {
        delete ops[id];
        delete logs[id];
      }
      return { ops, logs, selectedId: s.selectedId && ids.includes(s.selectedId) ? null : s.selectedId };
    }),
}));

type DoneListener = (op: Operation) => void;
const doneListeners = new Set<DoneListener>();

function payload<T>(ev: { data?: unknown }): T | undefined {
  const raw = Array.isArray(ev.data) ? ev.data[0] : ev.data;
  return raw as T | undefined;
}

let started = false;

/** Subscribes once for the app's lifetime and loads the initial list. */
function ensureStarted() {
  if (started) return;
  started = true;
  // Looking at an operation in the panel answers its failure toast: take the toast away.
  useOpsStore.subscribe((s) => {
    if (s.panelOpen && s.selectedId) toast.dismiss(opToastId(s.selectedId));
  });
  Events.On("operation:updated", (ev: { data?: unknown }) => {
    const op = payload<Operation>(ev);
    if (!op?.id) return;
    const st = useOpsStore.getState();
    const prev = st.ops[op.id];
    if (prev && prev.status !== OperationStatus.OpRunning && op.status === OperationStatus.OpRunning) return; // stale
    st.upsert(op);
    const justFinished = op.status !== OperationStatus.OpRunning && (!prev || prev.status === OperationStatus.OpRunning);
    if (!justFinished) return;
    if (op.status === OperationStatus.OpFailed && failureToastWanted(useOpsStore.getState(), op.id)) {
      toast.error(`${op.title} failed`, {
        id: opToastId(op.id),
        description: op.error || undefined,
        duration: Infinity,
        closeButton: true,
        action: { label: "View details", onClick: () => useOpsStore.getState().openPanel(op.id) },
      });
    }
    doneListeners.forEach((l) => {
      try {
        l(op);
      } catch {
        /* a listener must not break others */
      }
    });
  });
  Events.On("operation:log", (ev: { data?: unknown }) => {
    const l = payload<OperationLog>(ev);
    if (l?.id && l.lines?.length) useOpsStore.getState().appendLogs(l.id, l.lines);
  });
  OperationService.List()
    .then((list) => useOpsStore.getState().setAll(list ?? []))
    .catch((e) => useOpsStore.setState({ loaded: true, loadError: errorMessage(e) }));
}

export interface UseOperations {
  /** Newest first. */
  operations: Operation[];
  runningCount: number;
  finishedCount: number;
  loaded: boolean;
  loadError: string | null;
  /** Opens the Operations panel, on the given operation when an id is passed. */
  openOperations: (id?: string) => void;
  cancel: (id: string) => Promise<void>;
  clearFinished: () => Promise<void>;
}

export function useOperations(): UseOperations {
  useEffect(ensureStarted, []);
  const ops = useOpsStore((s) => s.ops);
  const loaded = useOpsStore((s) => s.loaded);
  const loadError = useOpsStore((s) => s.loadError);
  const openOperations = useOpsStore((s) => s.openPanel);
  const operations = useMemo(() => Object.values(ops).sort((a, b) => b.startedAt - a.startedAt), [ops]);
  const runningCount = operations.filter((o) => o.status === OperationStatus.OpRunning).length;
  return {
    operations,
    runningCount,
    finishedCount: operations.length - runningCount,
    loaded,
    loadError,
    openOperations,
    cancel: async (id) => {
      try {
        await OperationService.Cancel(id);
      } catch (e) {
        toast.error("Could not cancel operation", { description: errorMessage(e), duration: Infinity, closeButton: true });
      }
    },
    clearFinished: async () => {
      const ids = Object.values(useOpsStore.getState().ops)
        .filter((o) => o.status !== OperationStatus.OpRunning)
        .map((o) => o.id);
      try {
        await OperationService.ClearFinished();
        useOpsStore.getState().remove(ids);
      } catch (e) {
        toast.error("Could not clear finished operations", { description: errorMessage(e) });
      }
    },
  };
}

/** Log lines of one operation: loaded from the backend buffer when first requested, then appended from events. */
export function useOperationLogs(id: string, enabled: boolean): { lines: string[]; loading: boolean } {
  const lines = useOpsStore((s) => s.logs[id]);
  const requested = useRef(false);
  useEffect(() => {
    if (!enabled || requested.current) return;
    requested.current = true;
    // Mark as loaded-in-progress so events arriving now are appended to a defined buffer.
    useOpsStore.getState().setLogs(id, []);
    OperationService.Logs(id)
      .then((l) => useOpsStore.getState().setLogs(id, l ?? []))
      .catch(() => useOpsStore.getState().setLogs(id, ["(could not load logs)"]));
  }, [id, enabled]);
  return { lines: lines ?? [], loading: enabled && lines === undefined };
}

/**
 * Calls cb when an operation whose kind starts with kindPrefix (e.g. "image.")
 * on connectionId finishes (success, failure or cancel). Use it to refetch lists.
 */
export function useOnOperationDone(kindPrefix: string, connectionId: string | null, cb: (op: Operation) => void) {
  useEffect(ensureStarted, []);
  const ref = useRef(cb);
  ref.current = cb;
  useEffect(() => {
    if (!connectionId) return;
    const l: DoneListener = (op) => {
      if (op.connectionId === connectionId && op.kind.startsWith(kindPrefix)) ref.current(op);
    };
    doneListeners.add(l);
    return () => {
      doneListeners.delete(l);
    };
  }, [kindPrefix, connectionId]);
}

export const pastOperationsKey = (limit: number) => ["operations", "past", limit] as const;

/**
 * Persisted finished operations from previous sessions (metadata only, newest first).
 * Loaded on demand: pass enabled=false (or do not mount) until the History section is opened.
 * Refetches when a live operation finishes so the list never lags behind.
 */
export function usePastOperations(enabled = true, limit = 100) {
  const qc = useQueryClient();
  useEffect(ensureStarted, []);
  useEffect(() => {
    if (!enabled) return;
    const l: DoneListener = () => void qc.invalidateQueries({ queryKey: ["operations", "past"] });
    doneListeners.add(l);
    return () => {
      doneListeners.delete(l);
    };
  }, [enabled, qc]);
  return useQuery({
    queryKey: pastOperationsKey(limit),
    queryFn: async () => (await OperationService.PastOperations(limit)) ?? [],
    enabled,
    staleTime: 30_000,
    placeholderData: keepPreviousData,
    retry: false,
  });
}
