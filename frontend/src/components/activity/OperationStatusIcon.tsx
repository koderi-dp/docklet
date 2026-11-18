import { Ban, CheckCircle2, Loader2, XCircle } from "lucide-react";
import { OperationStatus } from "@bindings/model";

export function StatusIcon({ status }: { status: OperationStatus }) {
  switch (status) {
    case OperationStatus.OpRunning:
      return <Loader2 className="size-4 shrink-0 animate-spin text-info" aria-label="Running" />;
    case OperationStatus.OpSucceeded:
      return <CheckCircle2 className="size-4 shrink-0 text-success" aria-label="Succeeded" />;
    case OperationStatus.OpFailed:
      return <XCircle className="size-4 shrink-0 text-destructive" aria-label="Failed" />;
    default:
      return <Ban className="size-4 shrink-0 text-muted-foreground" aria-label="Cancelled" />;
  }
}
