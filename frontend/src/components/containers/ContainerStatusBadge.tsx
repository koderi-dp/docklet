import { AlertCircle, CheckCircle2 } from "lucide-react";
import { StatusBadge } from "@/components/shared/StatusBadge";
import { Badge } from "@/components/ui/badge";
import { isCompleted, isFailed } from "@/lib/composeGrouping";
import { cn } from "@/lib/utils";
import type { Container } from "@bindings/model";

/**
 * Engine state badge that understands one-shot containers: an exited container with code 0 and
 * restart policy "no" is "Completed" (muted); a non-zero exit shows "Exited (N)" (destructive).
 */
export function ContainerStatusBadge({ container, className }: { container: Pick<Container, "state" | "exitCode" | "restartPolicy">; className?: string }) {
  const base = "gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium";
  if (isCompleted(container)) {
    return (
      <Badge variant="outline" className={cn(base, "border-border bg-muted text-muted-foreground", className)} title="Exited with code 0 and will not restart">
        <CheckCircle2 className="size-3" aria-hidden />
        Completed
      </Badge>
    );
  }
  if (isFailed(container)) {
    return (
      <Badge variant="outline" className={cn(base, "border-destructive/30 bg-destructive/10 text-destructive", className)}>
        <AlertCircle className="size-3" aria-hidden />
        Exited ({container.exitCode})
      </Badge>
    );
  }
  return <StatusBadge state={container.state} className={className} />;
}
