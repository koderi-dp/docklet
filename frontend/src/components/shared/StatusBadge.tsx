import { Activity, AlertTriangle, HeartPulse, Loader2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type Tone = "success" | "warning" | "destructive" | "muted";

const TONE_BADGE: Record<Tone, string> = {
  success: "border-success/30 bg-success/10 text-success",
  warning: "border-warning/30 bg-warning/10 text-warning",
  destructive: "border-destructive/30 bg-destructive/10 text-destructive",
  muted: "border-border bg-muted text-muted-foreground",
};
const TONE_DOT: Record<Tone, string> = {
  success: "bg-success",
  warning: "bg-warning",
  destructive: "bg-destructive",
  muted: "bg-muted-foreground/60",
};

export function containerStateTone(state: string): Tone {
  switch (state) {
    case "running":
      return "success";
    case "paused":
    case "restarting":
    case "removing":
    case "created":
      return "warning";
    case "dead":
      return "destructive";
    default:
      return "muted"; // exited / unknown
  }
}

const STATE_LABEL: Record<string, string> = {
  running: "Running",
  exited: "Exited",
  created: "Created",
  paused: "Paused",
  restarting: "Restarting",
  removing: "Removing",
  dead: "Dead",
};

/** Small status dot used in the Name column. Color is never the only signal: the badge carries the text. */
export function StateDot({ state, className }: { state: string; className?: string }) {
  const tone = containerStateTone(state);
  return <span aria-hidden className={cn("inline-block size-2 shrink-0 rounded-full", TONE_DOT[tone], className)} />;
}

/** Exact Engine state: dot + text (spinner for transitional, warning icon for dead). */
export function StatusBadge({ state, className }: { state: string; className?: string }) {
  const tone = containerStateTone(state);
  const label = STATE_LABEL[state] ?? (state ? state.charAt(0).toUpperCase() + state.slice(1) : "Unknown");
  return (
    <Badge variant="outline" className={cn("gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium", TONE_BADGE[tone], className)}>
      {state === "restarting" || state === "removing" ? (
        <Loader2 className="size-3 animate-spin" aria-hidden />
      ) : state === "dead" ? (
        <AlertTriangle className="size-3" aria-hidden />
      ) : (
        <span aria-hidden className={cn("size-1.5 rounded-full", TONE_DOT[tone])} />
      )}
      {label}
    </Badge>
  );
}

/** Docker health. `none`/unknown renders an em dash with a "No healthcheck" tooltip — never "Healthy". */
export function HealthBadge({ health, className }: { health: string; className?: string }) {
  if (health === "healthy") {
    return (
      <Badge variant="outline" className={cn("gap-1 rounded-md px-2 py-0.5 text-xs font-medium", TONE_BADGE.success, className)}>
        <HeartPulse className="size-3" aria-hidden />
        Healthy
      </Badge>
    );
  }
  if (health === "unhealthy") {
    return (
      <Badge variant="outline" className={cn("gap-1 rounded-md px-2 py-0.5 text-xs font-medium", TONE_BADGE.destructive, className)}>
        <HeartPulse className="size-3" aria-hidden />
        Unhealthy
      </Badge>
    );
  }
  if (health === "starting") {
    return (
      <Badge variant="outline" className={cn("gap-1 rounded-md px-2 py-0.5 text-xs font-medium", TONE_BADGE.warning, className)}>
        <Activity className="size-3" aria-hidden />
        Starting
      </Badge>
    );
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className={cn("cursor-default text-muted-foreground", className)} aria-label="No healthcheck">
          —
        </span>
      </TooltipTrigger>
      <TooltipContent>No healthcheck</TooltipContent>
    </Tooltip>
  );
}
