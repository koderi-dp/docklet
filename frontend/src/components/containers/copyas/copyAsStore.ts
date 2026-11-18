import { create } from "zustand";
import { useUI } from "@/stores/uiStore";

export type CopyAsKind = "run" | "compose" | "connection";

interface CopyAsTarget {
  connectionId: string;
  containerId: string;
  name: string;
  kind: CopyAsKind;
}

interface CopyAsState {
  target: CopyAsTarget | null;
  close: () => void;
}

export const useCopyAsStore = create<CopyAsState>((set) => ({
  target: null,
  close: () => set({ target: null }),
}));

/** Opens a "Copy as…" dialog for a container of the active connection (rendered by CopyAsHost). */
export function openCopyAs(kind: CopyAsKind, containerId: string, name: string) {
  const connectionId = useUI.getState().activeConnectionId;
  if (connectionId) useCopyAsStore.setState({ target: { connectionId, containerId, name, kind } });
}
