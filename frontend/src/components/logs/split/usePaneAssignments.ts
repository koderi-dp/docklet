import { useCallback, useEffect, useRef, useState } from "react";
import { adaptPanes, LAYOUT_IDS, PANE_COUNT, reconcilePanes, setAll, setNames, type LayoutId, type PaneSource, type TickedItem } from "@/lib/logPanes";
import { DEFAULT_SPLIT, useLogSplitStore, type GroupSizes } from "@/stores/logSplitStore";

const NOTE_MS = 6000;

/** A container that was just placed into a pane because the user ticked it. */
export interface PaneNote {
  pane: number;
  text: string;
}

/**
 * Layout, pane assignments and divider sizes of the split view, kept per connection. Ticking and unticking
 * containers keeps the panes in step (see reconcilePanes); `ready` is false until the tick state is known, so
 * a restored layout is not pruned against an empty selection.
 */
export function usePaneAssignments(connectionId: string, items: readonly TickedItem[], ready: boolean) {
  const stored = useLogSplitStore((s) => s.byConnection[connectionId]) ?? DEFAULT_SPLIT;
  const patchStore = useLogSplitStore((s) => s.patch);
  const layout: LayoutId = LAYOUT_IDS.includes(stored.layout) ? stored.layout : "single";
  const [note, setNote] = useState<PaneNote | null>(null);
  const previous = useRef<{ connectionId: string; names: Set<string> | null }>({ connectionId, names: null });
  const itemsRef = useRef(items);
  itemsRef.current = items;

  const key = items.map((i) => `${i.project ?? ""}/${i.name}`).join("\n");
  useEffect(() => {
    if (!ready) return;
    if (previous.current.connectionId !== connectionId) previous.current = { connectionId, names: null };
    const state = useLogSplitStore.getState().byConnection[connectionId] ?? DEFAULT_SPLIT;
    const r = reconcilePanes(state.panes, itemsRef.current, previous.current.names);
    if (r.panes !== state.panes) patchStore(connectionId, { panes: r.panes });
    // Only a live tick is worth a note: on first load everything unassigned is simply distributed.
    if (previous.current.names !== null && r.placed.length > 0) setNote({ pane: r.placed[0].pane, text: r.placed.map((p) => p.name).join(", ") });
    previous.current.names = new Set(itemsRef.current.map((i) => i.name));
  }, [ready, key, connectionId, patchStore]);

  useEffect(() => {
    if (!note) return;
    const t = setTimeout(() => setNote(null), NOTE_MS);
    return () => clearTimeout(t);
  }, [note]);

  const setLayout = useCallback(
    (next: LayoutId) => {
      const state = useLogSplitStore.getState().byConnection[connectionId] ?? DEFAULT_SPLIT;
      const panes = next === "single" ? state.panes : adaptPanes(state.panes, PANE_COUNT[next], itemsRef.current);
      setNote(null);
      patchStore(connectionId, { layout: next, panes });
    },
    [connectionId, patchStore],
  );

  const changePane = useCallback(
    (index: number, fn: (p: PaneSource, ticked: string[]) => PaneSource) => {
      const state = useLogSplitStore.getState().byConnection[connectionId] ?? DEFAULT_SPLIT;
      const ticked = itemsRef.current.map((i) => i.name);
      patchStore(connectionId, { panes: state.panes.map((p, i) => (i === index ? fn(p, ticked) : p)) });
    },
    [connectionId, patchStore],
  );
  const setPaneAll = useCallback((index: number, on: boolean) => changePane(index, (p) => setAll(p, on)), [changePane]);
  const setPaneNames = useCallback((index: number, names: readonly string[], on: boolean) => changePane(index, (p, t) => setNames(p, t, names, on)), [changePane]);

  const setSizes = useCallback(
    (group: string, sizes: GroupSizes) => {
      const state = useLogSplitStore.getState().byConnection[connectionId] ?? DEFAULT_SPLIT;
      const cur = state.sizes[group];
      if (cur && Object.keys(sizes).length === Object.keys(cur).length && Object.entries(sizes).every(([k, v]) => Math.abs((cur[k] ?? -1) - v) < 0.05)) return;
      patchStore(connectionId, { sizes: { ...state.sizes, [group]: sizes } });
    },
    [connectionId, patchStore],
  );

  return { layout, panes: stored.panes, sizes: stored.sizes, note, setLayout, setPaneAll, setPaneNames, setSizes };
}
