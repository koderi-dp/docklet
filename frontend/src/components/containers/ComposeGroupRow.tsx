import { memo } from "react";
import { Box,ChevronRight, Layers, MoreHorizontal, Play, RotateCw, Square } from "lucide-react";
import { Link } from "react-router-dom";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { type ContainerAction, partitionByEligibility } from "@/lib/actionEligibility";
import { type CheckState, type ComposeGroup, type GroupState, oneShotNotes } from "@/lib/composeGrouping";
import type { IdleInfo } from "@/lib/idleTracker";
import type { ProjectTotals } from "@/lib/projectTotals";
import { cn } from "@/lib/utils";
import { CpuTotal, IdleChip, MemoryTotal } from "@/components/compose/ProjectTotalCells";
import { TriCheckbox } from "./TriCheckbox";
import type { GroupColumns } from "./useGroupColumns";

const STATE_LABEL: Record<GroupState, string> = {
  running: "Running",
  stopped: "Stopped",
  partial: "Partial",
  transitioning: "Transitioning",
};
const STATE_CLASS: Record<GroupState, string> = {
  running: "text-success",
  stopped: "text-muted-foreground",
  partial: "text-warning",
  transitioning: "text-warning",
};
const VERB: Record<"start" | "stop" | "restart", { label: string; past: string }> = {
  start: { label: "Start", past: "started" },
  stop: { label: "Stop", past: "stopped" },
  restart: { label: "Restart", past: "restarted" },
};

export interface ComposeGroupRowProps {
  group: ComposeGroup;
  collapsed: boolean;
  checkState: CheckState;
  busy: boolean;
  /** Connection capabilities for Compose CLI (never implied by labels alone). */
  canRunCompose: boolean;
  composeReason?: string;
  /** Which table columns exist right now, so the totals land in the CPU / Memory columns. */
  cols: GroupColumns;
  /** Live totals of the project's running containers (Compose groups only). */
  totals?: ProjectTotals;
  idle?: IdleInfo;
  handlers: GroupHandlers;
}

export interface GroupHandlers {
  onToggleCollapsed(g: ComposeGroup): void;
  onToggleSelect(g: ComposeGroup): void;
  /** Engine batch operation on the group's currently visible children. */
  onGroupAction(a: ContainerAction, g: ComposeGroup): void;
  onExpandAll(): void;
  onCollapseAll(): void;
}

export const ComposeGroupRow = memo(function ComposeGroupRow(p: ComposeGroupRowProps) {
  const { group: g } = p;
  const isCompose = g.kind === "compose";
  const n = g.all.length;
  const countText = g.filtered ? `${g.visible.length} of ${n} matching` : `${n} container${n === 1 ? "" : "s"}`;
  const label = isCompose ? (g.project as string) : "Standalone";
  const scopeNote = g.filtered ? "visible " : "";

  const actionBtn = (action: "start" | "stop" | "restart", icon: React.ReactNode) => {
    const v = VERB[action];
    const part = partitionByEligibility(action, g.visible);
    const reason = p.busy
      ? "An action is already in progress"
      : part.eligible.length === 0
        ? `No ${scopeNote}containers can be ${v.past}`
        : undefined;
    const count = part.eligible.length;
    const tip =
      reason ??
      `${v.label} ${count} ${scopeNote}container${count === 1 ? "" : "s"} in ${label}${part.skippedCount ? ` (${part.skippedCount} skipped)` : ""}`;
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="inline-flex">
            <Button
              variant="outline"
              size="icon"
              className="size-7"
              aria-label={`${v.label} ${label}`}
              disabled={!!reason}
              onClick={(e) => {
                e.stopPropagation();
                p.handlers.onGroupAction(action, g);
              }}
            >
              {icon}
            </Button>
          </span>
        </TooltipTrigger>
        <TooltipContent>{tip}</TooltipContent>
      </Tooltip>
    );
  };

  const composeDisabledReason = p.canRunCompose
    ? "Compose CLI actions are not available in this build"
    : (p.composeReason ?? "Compose CLI is not available for this connection");

  const menu = (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="icon" className="size-7" aria-label={`Group actions for ${label}`} onClick={(e) => e.stopPropagation()}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
            {!isCompose &&
              (["start", "stop", "restart"] as const).map((a) => {
                const part = partitionByEligibility(a, g.visible);
                const c = part.eligible.length;
                return (
                  <DropdownMenuItem key={a} disabled={p.busy || c === 0} onSelect={() => p.handlers.onGroupAction(a, g)}>
                    {a === "start" ? <Play /> : a === "stop" ? <Square /> : <RotateCw />}
                    {VERB[a].label} {c} container{c === 1 ? "" : "s"}
                  </DropdownMenuItem>
                );
              })}
            {isCompose && (
              <>
                <DropdownMenuItem asChild>
                  <Link to={`/compose/${encodeURIComponent(label)}`}>Open project</Link>
                </DropdownMenuItem>
                <DropdownMenuItem disabled title={composeDisabledReason}>
                  Compose Up / Down… (unavailable)
                </DropdownMenuItem>
                <DropdownMenuSeparator />
              </>
            )}
            <DropdownMenuItem onSelect={() => p.handlers.onToggleCollapsed(g)}>{p.collapsed ? "Expand" : "Collapse"}</DropdownMenuItem>
            <DropdownMenuItem onSelect={() => p.handlers.onExpandAll()}>Expand all</DropdownMenuItem>
            <DropdownMenuItem onSelect={() => p.handlers.onCollapseAll()}>Collapse all</DropdownMenuItem>
          
      </DropdownMenuContent>
    </DropdownMenu>
  );

  return (
    <tr className="h-[46px] cursor-pointer border-y border-border bg-primary/[0.07]" onClick={() => p.handlers.onToggleCollapsed(g)}>
      <td colSpan={p.cols.before} className="max-w-0 p-0">
        <div
          className="flex h-[46px] items-center gap-3 px-3 outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/60"
          role="button"
          tabIndex={0}
          aria-expanded={!p.collapsed}
          onKeyDown={(e) => {
            if (e.target !== e.currentTarget) return;
            if (e.key === " " || e.key === "Enter") {
              e.preventDefault();
              p.handlers.onToggleCollapsed(g);
            }
          }}
        >
      <TriCheckbox state={p.checkState} onToggle={() => p.handlers.onToggleSelect(g)} label={`Select visible containers in ${label}`} />
      <ChevronRight
        aria-hidden
        className={cn("size-4 shrink-0 text-muted-foreground transition-transform duration-150", !p.collapsed && "rotate-90")}
      />
      {isCompose ? (
        <Layers aria-hidden className="size-4 shrink-0 text-primary" />
      ) : (
        <Box aria-hidden className="size-4 shrink-0 text-muted-foreground" />
      )}
      {isCompose ? (
        <Link
          to={`/compose/${encodeURIComponent(label)}`}
          onClick={(e) => e.stopPropagation()}
          className="truncate text-sm font-semibold hover:underline"
          title={`Open project ${label}`}
        >
          {label}
        </Link>
      ) : (
        <span className="text-sm font-semibold">Standalone</span>
      )}
      {isCompose && (
        <Badge variant="outline" className="shrink-0 border-primary/40 bg-primary/15 text-[11px] font-medium text-primary">
          Compose Project
        </Badge>
      )}
      <span className="flex min-w-0 items-center gap-1.5 truncate text-xs text-muted-foreground">
        <span>{countText}</span>
        {isCompose && (
          <>
            <span aria-hidden>•</span>
            <span className={cn("font-medium", STATE_CLASS[g.state])}>{STATE_LABEL[g.state]}</span>
            {oneShotNotes(g).map((n) => (
              <span key={n.text} className={cn("font-normal", n.tone === "warning" ? "text-warning" : "text-muted-foreground")}>
                ({n.text})
              </span>
            ))}
          </>
        )}
        {g.health.withHealthcheck > 0 && (
          <>
            <span aria-hidden>•</span>
            <span className={cn(g.health.kind === "degraded" && "text-destructive")}>
              {g.health.healthy} of {g.health.withHealthcheck} healthy
            </span>
          </>
        )}
      </span>
      {isCompose && <IdleChip idle={p.idle} totals={p.totals} />}

      <div className="ml-auto flex shrink-0 items-center gap-1.5">
        {isCompose && (
          <>
            {actionBtn("start", <Play className="size-3.5" />)}
            {actionBtn("stop", <Square className="size-3.5" />)}
            {actionBtn("restart", <RotateCw className="size-3.5" />)}
          </>
        )}
      </div>
        </div>
      </td>
      {p.cols.cpu && (
        <td className="whitespace-nowrap px-3 text-right">{isCompose && <CpuTotal totals={p.totals} />}</td>
      )}
      {p.cols.memory && (
        <td className="whitespace-nowrap px-3 text-right">{isCompose && <MemoryTotal totals={p.totals} />}</td>
      )}
      <td colSpan={p.cols.after} className="whitespace-nowrap px-3 text-right">
        <div className="flex justify-end">{menu}</div>
      </td>
    </tr>
  );
});
