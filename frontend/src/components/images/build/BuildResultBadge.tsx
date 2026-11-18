import { Ban, Check, Loader2, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export type BuildResult = "success" | "failed" | "cancelled" | "running";

const TONE: Record<BuildResult, string> = {
  success: "border-success/30 bg-success/10 text-success",
  failed: "border-destructive/30 bg-destructive/10 text-destructive",
  cancelled: "border-border bg-muted text-muted-foreground",
  running: "border-info/30 bg-info/10 text-info",
};
const LABEL: Record<BuildResult, string> = { success: "Succeeded", failed: "Failed", cancelled: "Cancelled", running: "Running" };

export function BuildResultBadge({ result }: { result: string }) {
  const r = (result in TONE ? result : "cancelled") as BuildResult;
  return (
    <Badge variant="outline" className={cn("gap-1 rounded-md px-2 py-0.5 text-xs font-medium", TONE[r])}>
      {r === "running" ? <Loader2 className="animate-spin" aria-hidden /> : r === "success" ? <Check aria-hidden /> : r === "failed" ? <X aria-hidden /> : <Ban aria-hidden />}
      {LABEL[r]}
    </Badge>
  );
}

export function builderLabel(builder: string, method: string): string {
  return builder === "buildkit" ? `BuildKit · ${method}` : "Classic";
}
