import { Moon, Square } from "lucide-react";
import type { Container, ComposeProject } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Dash } from "@/components/containers/ContainerCells";
import { formatBytes } from "@/components/containers/format";
import { useComposeVerb, useProjectEngineBatch } from "@/hooks/useCompose";
import { formatIdleDuration, IDLE_CONFIG, type IdleInfo } from "@/lib/idleTracker";
import { partialNote, type ProjectTotals } from "@/lib/projectTotals";
import { cpuCores } from "@/lib/statsFormat";

function missing(t: ProjectTotals | undefined): string | undefined {
  if (!t || t.running === 0) return "No running containers";
  return "Waiting for the first stats sample";
}

/** Summed CPU of the project's running containers (percent of one core, like the per-container cell). */
export function CpuTotal({ totals }: { totals: ProjectTotals | undefined }) {
  const m = totals?.cpu;
  if (!m) return <Dash why={missing(totals)} />;
  const note = partialNote(m);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="tabular-nums">
          {m.value.toFixed(1)}%{note && <span className="text-muted-foreground">*</span>}
        </span>
      </TooltipTrigger>
      <TooltipContent>
        Sum of {m.reporting} running container{m.reporting === 1 ? "" : "s"}: about {cpuCores(m.value)} (100% = one core)
        {note && <div>{note}</div>}
      </TooltipContent>
    </Tooltip>
  );
}

/** Summed memory in use (no limit: limits differ per container). */
export function MemoryTotal({ totals }: { totals: ProjectTotals | undefined }) {
  const m = totals?.memory;
  if (!m) return <Dash why={missing(totals)} />;
  const note = partialNote(m);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="tabular-nums">
          {formatBytes(m.value)}
          {note && <span className="text-muted-foreground">*</span>}
        </span>
      </TooltipTrigger>
      <TooltipContent>
        Memory in use by {m.reporting} running container{m.reporting === 1 ? "" : "s"}
        {note && <div>{note}</div>}
      </TooltipContent>
    </Tooltip>
  );
}

const whenFmt = new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit" });

/** Quiet "Idle for N min" chip. In-memory only: the tooltip says so. */
export function IdleChip({ idle, totals }: { idle: IdleInfo | undefined; totals: ProjectTotals | undefined }) {
  if (!idle) return null;
  const mem = totals?.memory ? ` Memory in use: ${formatBytes(totals.memory.value)}.` : "";
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex shrink-0 items-center gap-1 rounded-md border border-border bg-muted px-1.5 py-0.5 text-[11px] font-normal text-muted-foreground">
          <Moon className="size-3" aria-hidden />
          Idle for {formatIdleDuration(idle.idleForMs)}
        </span>
      </TooltipTrigger>
      <TooltipContent className="max-w-xs">
        CPU has stayed under {IDLE_CONFIG.cpuThreshold}% since {whenFmt.format(idle.since)}.{mem} Based only on what Docklet has watched since it started; nothing is stored and it resets when Docklet restarts.
      </TooltipContent>
    </Tooltip>
  );
}

/** Project-page shortcut: goes through the same stop paths as the project actions (Compose CLI when available, Engine otherwise). */
export function StopIdleProject({ connectionId, project, containers }: { connectionId: string; project: ComposeProject; containers: readonly Container[] }) {
  const verbs = useComposeVerb(connectionId);
  const engine = useProjectEngineBatch(connectionId);
  const cli = project.capability.canRun;
  const busy = cli ? verbs.isBusy(project.name) : engine.pending;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex">
          <Button
            variant="outline"
            size="sm"
            className="h-6 gap-1 px-2 text-xs"
            disabled={busy}
            onClick={() => (cli ? void verbs.run("stop", project.name) : void engine.run("stop", [...containers]))}
          >
            <Square className="size-3" /> Stop project
          </Button>
        </span>
      </TooltipTrigger>
      <TooltipContent>{cli ? "Stop the project's services (docker compose stop)" : "Stop this project's containers through the Engine (Compose CLI is not available)"}</TooltipContent>
    </Tooltip>
  );
}
