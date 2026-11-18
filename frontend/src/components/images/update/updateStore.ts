import { create } from "zustand";
import { ImageService } from "@bindings/services";
import type { ImageRow, ImageUpdateStatus } from "@bindings/model";
import { errorMessage } from "@/lib/errors";

const key = (connectionId: string, ref: string) => `${connectionId}|${ref}`;

interface UpdateState {
  status: Record<string, ImageUpdateStatus>;
  checking: Record<string, boolean>;
  /** Image whose "Update…" dialog is open (rendered by UpdateHost). */
  target: { connectionId: string; ref: string } | null;
  setStatus: (connectionId: string, list: ImageUpdateStatus[]) => void;
  setChecking: (connectionId: string, refs: string[], on: boolean) => void;
  closeDialog: () => void;
}

export const useUpdateStore = create<UpdateState>((set) => ({
  status: {},
  checking: {},
  target: null,
  setStatus: (connectionId, list) =>
    set((s) => {
      const status = { ...s.status };
      for (const st of list) status[key(connectionId, st.ref)] = st;
      return { status };
    }),
  setChecking: (connectionId, refs, on) =>
    set((s) => {
      const checking = { ...s.checking };
      for (const r of refs) checking[key(connectionId, r)] = on;
      return { checking };
    }),
  closeDialog: () => set({ target: null }),
}));

const failed = (ref: string, e: unknown): ImageUpdateStatus => ({ ref, updateAvailable: false, checkedAt: Date.now(), error: errorMessage(e) });

/** Checks one image against its registry (always asks the registry). */
export async function checkImageUpdate(row: Pick<ImageRow, "connectionId" | "ref">) {
  const { setChecking, setStatus } = useUpdateStore.getState();
  setChecking(row.connectionId, [row.ref], true);
  try {
    setStatus(row.connectionId, [await ImageService.CheckUpdate(row.connectionId, row.ref)]);
  } catch (e) {
    setStatus(row.connectionId, [failed(row.ref, e)]);
  } finally {
    setChecking(row.connectionId, [row.ref], false);
  }
}

/** Checks every tagged image of the connection (the backend bounds concurrency and caches for 15 minutes). */
export async function checkAllImageUpdates(connectionId: string, rows: ImageRow[]) {
  const refs = [...new Set(rows.filter((r) => !r.dangling).map((r) => r.ref))];
  if (refs.length === 0) return;
  const { setChecking, setStatus } = useUpdateStore.getState();
  setChecking(connectionId, refs, true);
  try {
    setStatus(connectionId, (await ImageService.CheckUpdates(connectionId, refs)) ?? []);
  } catch (e) {
    setStatus(connectionId, refs.map((r) => failed(r, e)));
  } finally {
    setChecking(connectionId, refs, false);
  }
}

export function openUpdateDialog(row: Pick<ImageRow, "connectionId" | "ref">) {
  useUpdateStore.setState({ target: { connectionId: row.connectionId, ref: row.ref } });
}

export function useUpdateStatus(connectionId: string, ref: string) {
  const status = useUpdateStore((s) => s.status[key(connectionId, ref)]);
  const checking = useUpdateStore((s) => !!s.checking[key(connectionId, ref)]);
  return { status, checking };
}
