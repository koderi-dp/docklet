import { OperationStatus, type Operation } from "@bindings/model";

/**
 * The operation the panel shows. `live` (this session, newest first) and `earlier` (past sessions)
 * together are what the list offers. The wanted one wins while it is still listed; otherwise the
 * newest running operation, then the newest of this session, then the newest earlier one.
 */
export function pickSelected(wantedId: string | null, live: Operation[], earlier: Operation[]): Operation | null {
  if (wantedId) {
    const wanted = live.find((o) => o.id === wantedId) ?? earlier.find((o) => o.id === wantedId);
    if (wanted) return wanted;
  }
  return live.find((o) => o.status === OperationStatus.OpRunning) ?? live[0] ?? earlier[0] ?? null;
}
