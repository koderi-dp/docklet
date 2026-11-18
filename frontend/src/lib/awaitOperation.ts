import { Events } from "@wailsio/runtime";
import { OperationService } from "@bindings/services";
import { OperationStatus, type Operation } from "@bindings/model";

/** Resolves with the final state of a background operation (succeeded, failed or cancelled). */
export function awaitOperation(opId: string): Promise<Operation> {
  return new Promise((resolve) => {
    let finished = false;
    const finish = (op: Operation) => {
      if (finished) return;
      finished = true;
      off();
      resolve(op);
    };
    const off = Events.On("operation:updated", (ev: { data?: unknown }) => {
      const op = (Array.isArray(ev.data) ? ev.data[0] : ev.data) as Operation | undefined;
      if (op?.id === opId && op.status !== OperationStatus.OpRunning) finish(op);
    });
    // The operation may have finished before the subscription existed.
    OperationService.List()
      .then((list) => {
        const op = list?.find((o) => o.id === opId);
        if (op && op.status !== OperationStatus.OpRunning) finish(op);
      })
      .catch(() => undefined);
  });
}
