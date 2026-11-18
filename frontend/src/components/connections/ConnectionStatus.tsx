import { Loader2 } from "lucide-react";
import { ConnectionState } from "@bindings/model";
import { cn } from "@/lib/utils";

export function ConnectionDot({ state, className }: { state?: ConnectionState; className?: string }) {
  if (state === ConnectionState.StateConnecting)
    return <Loader2 className={cn("size-3 animate-spin text-warning", className)} aria-label="Connecting" />;
  const color =
    state === ConnectionState.StateConnected
      ? "bg-success"
      : state === ConnectionState.StateError
        ? "bg-destructive"
        : "bg-muted-foreground/60";
  return <span className={cn("inline-block size-2 rounded-full", color, className)} aria-hidden />;
}
