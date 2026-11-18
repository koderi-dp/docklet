import { Loader2 } from "lucide-react";
import { ConnectionState } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { stateText, statusTone } from "./connectionMeta";

const TONE: Record<string, string> = {
  success: "border-success/30 bg-success/10 text-success",
  warning: "border-warning/30 bg-warning/10 text-warning",
  destructive: "border-destructive/30 bg-destructive/10 text-destructive",
  muted: "border-border bg-muted text-muted-foreground",
};
const DOT: Record<string, string> = {
  success: "bg-success",
  warning: "bg-warning",
  destructive: "bg-destructive",
  muted: "bg-muted-foreground/60",
};

export function ConnectionStatusBadge({ state, className }: { state?: ConnectionState; className?: string }) {
  const tone = statusTone(state);
  return (
    <Badge variant="outline" className={cn("gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium", TONE[tone], className)}>
      {state === ConnectionState.StateConnecting ? (
        <Loader2 className="size-3 animate-spin" aria-hidden />
      ) : (
        <span aria-hidden className={cn("size-1.5 rounded-full", DOT[tone])} />
      )}
      {stateText(state)}
    </Badge>
  );
}
