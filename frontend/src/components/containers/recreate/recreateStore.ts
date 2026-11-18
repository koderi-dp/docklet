import { create } from "zustand";
import { useUI } from "@/stores/uiStore";

interface RecreateTarget {
  connectionId: string;
  containerId: string;
}

interface RecreateState {
  target: RecreateTarget | null;
  close: () => void;
}

export const useRecreateStore = create<RecreateState>((set) => ({
  target: null,
  close: () => set({ target: null }),
}));

/** Opens "Edit & recreate" for a container of the active connection (rendered by RecreateHost). */
export function openRecreate(containerId: string) {
  const connectionId = useUI.getState().activeConnectionId;
  if (connectionId) useRecreateStore.setState({ target: { connectionId, containerId } });
}
