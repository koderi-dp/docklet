import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { LayoutId, PaneSource } from "@/lib/logPanes";

/** Sizes of one resizable group: panel id to percentage. */
export type GroupSizes = Record<string, number>;

/** Split view of the Logs screen for one connection. Containers are kept by name, so they survive recreation. */
export interface SplitState {
  layout: LayoutId;
  /** One entry per pane of the last split layout (kept while the layout is Single). */
  panes: PaneSource[];
  /** Divider positions per resizable group. */
  sizes: Record<string, GroupSizes>;
}

export const DEFAULT_SPLIT: SplitState = { layout: "single", panes: [], sizes: {} };

interface Store {
  byConnection: Record<string, SplitState>;
  patch: (connectionId: string, patch: Partial<SplitState>) => void;
}

export const useLogSplitStore = create<Store>()(
  persist(
    (set) => ({
      byConnection: {},
      patch: (connectionId, patch) =>
        set((s) => ({ byConnection: { ...s.byConnection, [connectionId]: { ...(s.byConnection[connectionId] ?? DEFAULT_SPLIT), ...patch } } })),
    }),
    { name: "docklet-log-split" },
  ),
);
