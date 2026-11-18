import { useState } from "react";
import { Eye, Square } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useNow, useWatchActions, useWatchAvailability } from "@/hooks/useComposeWatch";
import { cn } from "@/lib/utils";
import type { ComposeProject } from "@bindings/model";
import { WatchHelp } from "./WatchHelp";
import { WatchStartPanel } from "./WatchStartPanel";
import { formatElapsed } from "./watchFormat";

/** Quiet running indicator: pulsing dot, "Watching" (or "Starting watch") and the real elapsed time. */
export function WatchChip({ ready, startedAt, now, compact }: { ready: boolean; startedAt: number; now: number; compact?: boolean }) {
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap px-2 text-xs text-muted-foreground" data-testid="watch-chip">
      <span className="relative flex size-2">
        <span className={cn("absolute inline-flex size-full animate-ping rounded-full opacity-60", ready ? "bg-success" : "bg-warning")} />
        <span className={cn("relative inline-flex size-2 rounded-full", ready ? "bg-success" : "bg-warning")} />
      </span>
      <span className={cn("text-foreground", compact && "hidden xl:inline")}>{ready ? "Watching" : "Starting watch"}</span>
      <span className="tabular-nums">{formatElapsed(now - startedAt)}</span>
    </span>
  );
}

/** Menu entry that starts the watch flow on narrow windows (the inline button is hidden there). */
export function WatchMenuItem({ connectionId, project, onChoose }: { connectionId: string; project: ComposeProject; onChoose: () => void }) {
  const a = useWatchAvailability(connectionId, project);
  if (a.running) return null;
  return (
    <DropdownMenuItem className="xl:hidden" disabled={!!a.reason} title={a.reason ?? "Watch files and sync, rebuild or restart services (docker compose watch)"} onSelect={onChoose}>
      <Eye /> Watch…
    </DropdownMenuItem>
  );
}

/**
 * Header control. Running: Watching chip + Stop (compact below xl). Idle: a "Watch" button with the start
 * panel on wide windows; on narrow ones the same flow opens from the ⋯ menu (WatchMenuItem) as a dialog.
 */
export function WatchHeaderControl({
  connectionId,
  project,
  dialogOpen,
  onDialogOpenChange,
}: {
  connectionId: string;
  project: ComposeProject;
  dialogOpen: boolean;
  onDialogOpenChange: (open: boolean) => void;
}) {
  const { info, state, reason, noRules, running } = useWatchAvailability(connectionId, project);
  const actions = useWatchActions(connectionId, project.name);
  const now = useNow(running);

  if (running && state.data) {
    return (
      <div className="inline-flex h-8 items-center rounded-md border border-border bg-background">
        <WatchChip ready={state.data.ready} startedAt={state.data.startedAt ?? now} now={now} compact />
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="sm"
              className="h-full rounded-l-none border-l border-border px-2"
              disabled={actions.busy}
              aria-label={`Stop watching ${project.name}`}
              onClick={() => void actions.stopWatch()}
            >
              <Square className="size-3.5" /> <span className="hidden xl:inline">Stop</span>
            </Button>
          </TooltipTrigger>
          <TooltipContent>Stops watching. Containers keep running.</TooltipContent>
        </Tooltip>
      </div>
    );
  }

  const panel = (close: () => void) =>
    info.data && (
      <WatchStartPanel
        project={project.name}
        services={info.data.services ?? []}
        initial={state.data?.active ? { services: state.data.services ?? [], noUp: state.data.noUp } : undefined}
        onStart={(o) => {
          close();
          void actions.startWatch(o);
        }}
      />
    );

  return (
    <>
      <InlineWatch reason={reason} busy={actions.busy} name={project.name} panel={panel} />
      {noRules && <WatchHelp project={project.name} />}
      <Dialog open={dialogOpen} onOpenChange={onDialogOpenChange}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle className="sr-only">Watch {project.name}</DialogTitle>
          </DialogHeader>
          {panel(() => onDialogOpenChange(false))}
        </DialogContent>
      </Dialog>
    </>
  );
}

function InlineWatch({ reason, busy, name, panel }: { reason?: string; busy: boolean; name: string; panel: (close: () => void) => React.ReactNode }) {
  const [open, setOpen] = useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="hidden xl:inline-flex">
            <PopoverTrigger asChild>
              <Button variant="outline" size="sm" disabled={!!reason || busy} aria-label={`Compose watch ${name}`}>
                <Eye className="size-3.5" /> Watch
              </Button>
            </PopoverTrigger>
          </span>
        </TooltipTrigger>
        <TooltipContent className="max-w-sm">{reason ?? "Watch files and sync, rebuild or restart services (docker compose watch)"}</TooltipContent>
      </Tooltip>
      <PopoverContent align="end" className="w-80">
        {panel(() => setOpen(false))}
      </PopoverContent>
    </Popover>
  );
}
