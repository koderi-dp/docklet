import { toast } from "sonner";
import { OperationStatus } from "@bindings/model";
import { useOnOperationDone } from "@/hooks/useOperations";
import { useUI } from "@/stores/uiStore";
import { RecreateContainerDialog } from "./RecreateContainerDialog";
import { useRecreateStore } from "./recreateStore";

/** Mount once on pages that offer "Edit & recreate…": renders the dialog and reports successes. */
export function RecreateHost() {
  const target = useRecreateStore((s) => s.target);
  const close = useRecreateStore((s) => s.close);
  const connectionId = useUI((s) => s.activeConnectionId);
  useOnOperationDone("container.recreate", connectionId, (op) => {
    if (op.status === OperationStatus.OpSucceeded) toast.success(`${op.title} finished`);
  });
  if (!target) return null;
  return <RecreateContainerDialog key={target.containerId} connectionId={target.connectionId} containerId={target.containerId} onClose={close} />;
}
