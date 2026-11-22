import { useEffect, useMemo, useRef } from "react";
import { Link } from "react-router-dom";
import { Layers, Play, RotateCw, Square } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ContainerStatusBadge } from "@/components/containers/ContainerStatusBadge";
import { HealthBadge, StateDot } from "@/components/shared/StatusBadge";
import { publishedPorts } from "@/components/containers/format";
import { useComposeVerb, useProjectEngineBatch } from "@/hooks/useCompose";
import { partitionByEligibility } from "@/lib/actionEligibility";
import type { ComposeProject, Container } from "@bindings/model";
import { ProjectStateBadge } from "./ProjectStateBadge";

type Act = "start" | "stop" | "restart";
const ICON: Record<Act, React.ReactNode> = { start: <Play />, stop: <Square />, restart: <RotateCw /> };
const LABEL: Record<Act, string> = { start: "Start", stop: "Stop", restart: "Restart" };

/**
 * Engine actions for one container or all containers of a service. Only actions that apply to the
 * current state are shown (a running container has no Start button), so a row never carries
 * look-alike icons that do nothing.
 */
function ActionButtons({ targets, name, busy, onAct, omit }: { targets: Container[]; name: string; busy: boolean; onAct: (a: Act) => void; omit?: Act }) {
  return (
    <div className="flex justify-end gap-1">
      {(["start", "stop", "restart"] as const).map((a) => {
        const eligible = partitionByEligibility(a, targets).eligible.length;
        if (a === omit || eligible === 0) return null;
        const reason = busy ? "Another action is in progress" : undefined;
        return (
          <Tooltip key={a}>
            <TooltipTrigger asChild>
              <span className="inline-flex">
                <Button variant="ghost" size="icon" className="size-7" disabled={!!reason} aria-label={`${LABEL[a]} ${name}`} onClick={() => onAct(a)}>
                  <span className="inline-flex [&_svg]:size-3.5">{ICON[a]}</span>
                </Button>
              </span>
            </TooltipTrigger>
            <TooltipContent>{reason ?? (targets.length === 1 ? LABEL[a] : `${LABEL[a]} ${eligible} container${eligible === 1 ? "" : "s"}`)}</TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}

/** Hierarchical service -> container table. Actions are Engine operations via ContainerService. */
export function ServicesTab({ connectionId, project, containers, focusService }: { connectionId: string; project: ComposeProject; containers: Container[]; focusService?: string }) {
  const engine = useProjectEngineBatch(connectionId);
  const verbs = useComposeVerb(connectionId);
  const cap = project.capability;
  const upReason = !cap.canRun ? cap.reason || "Compose CLI is not available for this project" : verbs.isBusy(project.name) ? "A Compose command is being submitted" : undefined;
  const groups = useMemo(() => {
    const by = new Map<string, Container[]>();
    for (const c of containers) {
      const k = c.composeService || "";
      by.set(k, [...(by.get(k) ?? []), c]);
    }
    const order = (project.services ?? []).map((s) => s.name);
    // Services defined in the Compose files but without a container yet are listed too.
    const names = [...new Set([...order, ...by.keys()])].filter((n) => n !== "" || by.has(n));
    return names.map((n) => ({ name: n, list: (by.get(n) ?? []).sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true })) }));
  }, [containers, project.services]);

  const wrapRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!focusService) return;
    const row = Array.from(wrapRef.current?.querySelectorAll<HTMLElement>("[data-service]") ?? []).find((r) => r.dataset.service === focusService);
    if (!row) return;
    row.scrollIntoView({ block: "center", behavior: "smooth" });
    const ring = ["ring-2", "ring-inset", "ring-primary"];
    row.classList.add(...ring);
    const t = window.setTimeout(() => row.classList.remove(...ring), 2000);
    return () => {
      window.clearTimeout(t);
      row.classList.remove(...ring);
    };
  }, [focusService]);

  if (groups.length === 0) return <p className="p-4 text-muted-foreground">No containers carry this project label{project.managed ? " and the Compose files define no services we could read" : ""}.</p>;

  return (
    <div ref={wrapRef} className="overflow-x-auto rounded-[10px] border border-border bg-card">
      <table className="w-full min-w-[720px] border-collapse text-[13px]">
        <thead>
          <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
            <th className="h-10 px-3 font-medium">Service / container</th>
            <th className="px-3 font-medium">Image</th>
            <th className="px-3 font-medium">Status</th>
            <th className="px-3 font-medium">Health</th>
            <th className="px-3 font-medium">Ports</th>
            <th className="px-3 text-right font-medium">
              <span className="sr-only">Actions</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {groups.map((g) => {
            const running = g.list.filter((c) => c.state === "running").length;
            return (
              <ServiceRows
                key={g.name}
                name={g.name}
                list={g.list}
                running={running}
                busy={engine.pending}
                onAct={(a, t) => void engine.run(a, t)}
                upReason={g.name ? upReason : "Containers without a service label cannot be started with Compose"}
                composeAvailable={cap.canRun && !!g.name}
                onUp={() => void verbs.run("up", project.name, { service: g.name })}
              />
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function ServiceRows({
  name,
  list,
  running,
  busy,
  onAct,
  upReason,
  composeAvailable,
  onUp,
}: {
  name: string;
  list: Container[];
  running: number;
  busy: boolean;
  onAct: (a: Act, t: Container[]) => void;
  upReason?: string;
  /** False when Compose cannot run for this service: existing containers then get plain Start instead of Up. */
  composeAvailable: boolean;
  onUp: () => void;
}) {
  // Up is shown when it would do something. Without Compose it stays only for a service that has no
  // container yet, disabled, so the reason can be read.
  const needsUp = list.length === 0 || (running < list.length && composeAvailable);
  return (
    <>
      <tr data-service={name} className="h-[42px] border-y border-border bg-primary/[0.07]">
        <td className="px-3" colSpan={2}>
          <div className="flex items-center gap-2">
            <Layers aria-hidden className="size-4 text-primary" />
            <span className="font-semibold">{name || "—"}</span>
            <span className="text-xs text-muted-foreground">
              service · {running} of {list.length} running
            </span>
          </div>
        </td>
        <td className="px-3">
          {list.length === 0 ? (
            <Badge variant="outline" className="rounded-md border-dashed px-2 py-0.5 text-xs font-medium text-muted-foreground">
              Not created
            </Badge>
          ) : (
            <ProjectStateBadge containers={list} />
          )}
        </td>
        <td />
        <td />
        <td className="px-3">
          <div className="flex items-center justify-end gap-1">
            {needsUp && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="inline-flex">
                    <Button variant="outline" size="sm" className="h-7 gap-1.5 px-2 text-xs" disabled={!!upReason} aria-label={`Compose Up service ${name}`} onClick={onUp}>
                      <Play className="size-3.5" /> Up
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent className="max-w-sm">{upReason ?? `Create and start this service (docker compose up -d ${name})`}</TooltipContent>
              </Tooltip>
            )}
            {/* Up already starts stopped containers; plain Start is only offered when Compose cannot run. */}
            {list.length > 0 && <ActionButtons targets={list} name={`service ${name}`} busy={busy} omit={composeAvailable ? "start" : undefined} onAct={(a) => onAct(a, list)} />}
          </div>
        </td>
      </tr>
      {list.map((c) => {
        const ports = publishedPorts(c.ports ?? []);
        return (
          <tr key={c.id} className="h-[42px] border-b border-border/60 hover:bg-accent/40">
            <td className="px-3 pl-9">
              <div className="flex min-w-0 items-center gap-2">
                <StateDot state={c.state} />
                <Link to={`/containers/${c.id}`} className="truncate font-medium hover:underline" title={c.name}>
                  {c.name}
                </Link>
              </div>
            </td>
            <td className="px-3">
              <span className="block max-w-[220px] truncate text-muted-foreground" title={c.image}>
                {c.image}
              </span>
            </td>
            <td className="px-3">
              <ContainerStatusBadge container={c} />
            </td>
            <td className="px-3">
              <HealthBadge health={c.health} />
            </td>
            <td className="px-3">
              {ports.length ? (
                <span className="font-mono text-xs" title={ports.join(", ")}>
                  {ports.join(", ")}
                </span>
              ) : (
                <span className="text-muted-foreground">—</span>
              )}
            </td>
            <td className="px-3">
              <ActionButtons targets={[c]} name={c.name} busy={busy} onAct={(a) => onAct(a, [c])} />
            </td>
          </tr>
        );
      })}
    </>
  );
}
