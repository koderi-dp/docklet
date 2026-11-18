import { Link } from "react-router-dom";
import { Box, MoreHorizontal, Pause, Pencil, Play, RotateCw, Skull, Square, Trash2 } from "lucide-react";
import type { Container } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { HealthBadge } from "@/components/shared/StatusBadge";
import { ACTION_LABEL, ineligibleReason, type ContainerAction } from "@/lib/actionEligibility";
import type { ReactNode } from "react";
import { openRecreate } from "@/components/containers/recreate/recreateStore";
import { ContainerStatusBadge } from "./ContainerStatusBadge";

interface Props {
  container: Container;
  health?: Container["health"];
  busy: boolean;
  onRun: (action: ContainerAction) => void;
  onConfirm: (what: "kill" | "remove") => void;
}

const ICON: Partial<Record<ContainerAction, ReactNode>> = {
  start: <Play />,
  stop: <Square />,
  restart: <RotateCw />,
  pause: <Pause />,
  unpause: <Play />,
};

/** Breadcrumb, title + status, and one row of actions: Start/Stop/Restart plus an overflow menu. */
export function ContainerDetailHeader({ container: c, health, busy, onRun, onConfirm }: Props) {
  const reasonFor = (a: ContainerAction) => (busy ? "Another action is in progress" : ineligibleReason(a, c.state));

  const primary = (action: ContainerAction) => {
    const reason = reasonFor(action);
    const b = (
      <Button size="sm" variant="outline" disabled={!!reason} onClick={() => onRun(action)}>
        <span className="[&_svg]:size-3.5">{ICON[action]}</span>
        {ACTION_LABEL[action]}
      </Button>
    );
    return reason ? (
      <Tooltip>
        <TooltipTrigger asChild><span>{b}</span></TooltipTrigger>
        <TooltipContent>{reason}</TooltipContent>
      </Tooltip>
    ) : b;
  };

  const pauseAction: ContainerAction = c.state === "paused" ? "unpause" : "pause";

  return (
    <>
      <nav aria-label="Breadcrumb" className="mb-2 text-xs text-muted-foreground">
        <Link to="/containers" className="rounded hover:text-foreground hover:underline focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none">Containers</Link>
        <span className="mx-1.5">/</span>
        <span className="text-foreground">{c.name}</span>
      </nav>
      <div className="mb-3 flex flex-nowrap items-center gap-3">
        <Box className="size-6 shrink-0 text-primary" />
        <h1 className="min-w-0 truncate text-2xl font-semibold" title={c.name}>{c.name}</h1>
        <ContainerStatusBadge container={c} />
        <HealthBadge health={health ?? c.health} />
        <div className="ml-auto flex shrink-0 items-center gap-2">
          {primary("start")}
          {primary("stop")}
          {primary("restart")}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button size="icon" variant="outline" className="size-8" aria-label="More actions" title="More actions">
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem disabled={!!reasonFor(pauseAction)} title={reasonFor(pauseAction)} onSelect={() => onRun(pauseAction)}>
                {ICON[pauseAction]} {ACTION_LABEL[pauseAction]}
              </DropdownMenuItem>
              <DropdownMenuItem disabled={busy} title={busy ? "Another action is in progress" : "Change settings and replace the container"} onSelect={() => openRecreate(c.id)}>
                <Pencil /> Edit & recreate…
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" disabled={!!reasonFor("kill")} title={reasonFor("kill")} onSelect={() => onConfirm("kill")}>
                <Skull /> {ACTION_LABEL.kill}…
              </DropdownMenuItem>
              <DropdownMenuItem variant="destructive" disabled={!!reasonFor("remove")} title={reasonFor("remove")} onSelect={() => onConfirm("remove")}>
                <Trash2 /> {ACTION_LABEL.remove}…
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </>
  );
}
