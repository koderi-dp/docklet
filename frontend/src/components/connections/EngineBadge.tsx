import type { EngineKind } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { isPodmanKind } from "@/lib/engineKind";
import { cn } from "@/lib/utils";

/** A subtle "Podman" marker. Docker is the default and gets no badge. */
export function EngineBadge({ kind, className }: { kind?: EngineKind | string; className?: string }) {
  if (!isPodmanKind(kind)) return null;
  return (
    <Badge variant="outline" className={cn("shrink-0 rounded-md px-1.5 py-0 text-[11px] font-normal text-muted-foreground", className)}>
      Podman
    </Badge>
  );
}
