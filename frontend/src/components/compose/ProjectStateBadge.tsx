import { Loader2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { aggregateHealth, aggregateState, countOneShots, oneShotNotes, type GroupState } from "@/lib/composeGrouping";
import { cn } from "@/lib/utils";
import type { Container } from "@bindings/model";

const LABEL: Record<GroupState, string> = { running: "Running", stopped: "Stopped", partial: "Partial", transitioning: "Transitioning" };
const TONE: Record<GroupState, string> = {
  running: "border-success/30 bg-success/10 text-success",
  stopped: "border-border bg-muted text-muted-foreground",
  partial: "border-warning/30 bg-warning/10 text-warning",
  transitioning: "border-warning/30 bg-warning/10 text-warning",
};

/** Aggregate state using the shared grouping semantics (lib/composeGrouping). */
export function ProjectStateBadge({
  containers,
  className,
  notCreated,
}: {
  containers: readonly Container[];
  className?: string;
  /** Managed project with no containers yet: shown as "Not created", never as "Stopped". */
  notCreated?: boolean;
}) {
  const state = aggregateState(containers);
  if (notCreated) {
    return (
      <Badge variant="outline" className={cn("gap-1.5 rounded-md border-dashed px-2 py-0.5 text-xs font-medium text-muted-foreground", className)}>
        Not created
      </Badge>
    );
  }
  const notes = oneShotNotes(countOneShots(containers));
  return (
    <span className="inline-flex items-center gap-1.5">
      <Badge variant="outline" className={cn("gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium", TONE[state], className)}>
        {state === "transitioning" && <Loader2 className="size-3 animate-spin" aria-hidden />}
        {LABEL[state]}
      </Badge>
      {notes.map((n) => (
        <span key={n.text} className={cn("text-xs", n.tone === "warning" ? "text-warning" : "text-muted-foreground")}>
          {n.text}
        </span>
      ))}
    </span>
  );
}

/** "2 of 3 healthy" computed only from real health signals; an em dash when no healthchecks exist. */
export function ProjectHealthText({ containers }: { containers: readonly Container[] }) {
  const h = aggregateHealth(containers);
  if (h.withHealthcheck === 0) return <span className="text-muted-foreground">—</span>;
  return (
    <span className={cn("text-xs", h.kind === "degraded" ? "text-destructive" : "text-muted-foreground")}>
      {h.healthy} of {h.withHealthcheck} healthy
    </span>
  );
}
