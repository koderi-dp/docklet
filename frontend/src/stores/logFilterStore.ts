import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { IncludeMode, Rule } from "@/lib/logFilter/rules";

/** A named rule set. Global: not tied to a connection. */
export interface SavedFilter {
  id: string;
  name: string;
  rules: Rule[];
  includeMode: IncludeMode;
}

/** The rule set that was open last, restored when the Logs screen opens again. */
export interface LastFilter {
  rules: Rule[];
  includeMode: IncludeMode;
  activeSavedId: string | null;
}

interface LogFilterState {
  saved: SavedFilter[];
  last: LastFilter;
  /** The container list of the Logs screen is folded away. */
  panelCollapsed: boolean;
  setSaved: (saved: SavedFilter[]) => void;
  setLast: (last: LastFilter) => void;
  setPanelCollapsed: (v: boolean) => void;
}

export const useLogFilterStore = create<LogFilterState>()(
  persist(
    (set) => ({
      saved: [],
      last: { rules: [], includeMode: "any", activeSavedId: null },
      panelCollapsed: false,
      setSaved: (saved) => set({ saved }),
      setLast: (last) => set({ last }),
      setPanelCollapsed: (panelCollapsed) => set({ panelCollapsed }),
    }),
    { name: "docklet-log-filters" },
  ),
);
