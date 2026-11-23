import { useState } from "react";
import { ArrowDownToLine, Download, EllipsisVertical, Hammer, Layers, Play, RotateCw, Square, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useComposeVerb, useProjectEngineBatch, type ComposeVerb, type UpOptions } from "@/hooks/useCompose";
import { partitionByEligibility } from "@/lib/actionEligibility";
import { aggregateState } from "@/lib/composeGrouping";
import type { ComposeProject, Container } from "@bindings/model";
import { useProjectFolderMenu } from "@/components/host/useProjectFolderMenu";
import { DownDialog } from "./DownDialog";
import { ForgetProjectDialog } from "./ForgetProjectDialog";
import { UpOptionsPopover } from "./UpOptionsPopover";
import { WatchHeaderControl, WatchMenuItem } from "./watch/WatchHeaderControl";

interface Props {
  connectionId: string;
  project: ComposeProject;
  containers: readonly Container[];
  /** Header variant shows text labels, table rows a labelled Start/Stop button plus the overflow menu. */
  labels?: boolean;
  /** Called after a managed project was forgotten (e.g. to leave its page). */
  onForgotten?: () => void;
}

const VERBS: { verb: ComposeVerb; label: string; icon: React.ReactNode; hint: string }[] = [
  { verb: "up", label: "Up", icon: <Play />, hint: "Create and start the project's services (docker compose up -d)" },
  { verb: "down", label: "Down", icon: <ArrowDownToLine />, hint: "Remove the project's containers and networks (docker compose down)" },
  { verb: "restart", label: "Restart", icon: <RotateCw />, hint: "Restart the project's services (docker compose restart)" },
  { verb: "pull", label: "Pull", icon: <Download />, hint: "Pull service images (docker compose pull)" },
  { verb: "build", label: "Build", icon: <Hammer />, hint: "Build service images (docker compose build)" },
];

/**
 * Compose CLI verbs are enabled only when the project's capability says the CLI can run;
 * Engine batch start/stop/restart of containers is always available and labelled as such.
 */
export function ProjectActions({ connectionId, project, containers, labels, onForgotten }: Props) {
  const verbs = useComposeVerb(connectionId);
  const engine = useProjectEngineBatch(connectionId);
  const [confirmDown, setConfirmDown] = useState(false);
  const [confirmForget, setConfirmForget] = useState(false);
  const [watchOpen, setWatchOpen] = useState(false);
  const cap = project.capability;
  const busy = verbs.isBusy(project.name);
  const folderItems = useProjectFolderMenu(connectionId, project.workingDir, project.configFiles ?? []);

  const notCreated = project.notCreated;
  const NOT_CREATED = "The project has not been created yet: there is nothing to act on. Use Up first.";
  const baseReason = !cap.canRun ? cap.reason || "Compose CLI is not available for this project" : busy ? "A Compose command is being submitted" : undefined;
  const upOpts = (o: UpOptions) => void verbs.run("up", project.name, { up: o });

  const onVerb = (verb: ComposeVerb) => (verb === "down" ? setConfirmDown(true) : void verbs.run(verb, project.name));

  const dialogs = (
    <>
      <ForgetProjectDialog
        connectionId={connectionId}
        project={project.name}
        files={project.configFiles ?? []}
        open={confirmForget}
        onOpenChange={setConfirmForget}
        onForgotten={onForgotten}
      />
      <DownDialog
        connectionId={connectionId}
        project={project.name}
        containers={containers}
        open={confirmDown}
        onOpenChange={setConfirmDown}
        onConfirm={(o) => {
          setConfirmDown(false);
          void verbs.run("down", project.name, { volumes: o.volumes });
        }}
      />
    </>
  );

  // Table-row variant: a labelled Start/Stop button + a single overflow menu with everything else (fixed-width column).
  if (!labels) {
    const reasonFor = (verb: ComposeVerb) =>
      baseReason ?? (notCreated && (verb === "down" || verb === "restart") ? NOT_CREATED : undefined);
    const stopping = !notCreated && aggregateState(containers) === "running";
    const primaryReason = stopping ? baseReason : reasonFor("up");
    const primaryHint = stopping ? "Stop the project's services (docker compose stop)" : VERBS[0].hint;
    return (
      <div className="flex flex-nowrap items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="inline-flex">
              <Button
                variant="outline"
                size="sm"
                className="h-7 gap-1 px-2.5 text-xs"
                disabled={!!primaryReason}
                aria-label={`${stopping ? "Stop" : "Start"} ${project.name}`}
                onClick={() => (stopping ? void verbs.run("stop", project.name) : onVerb("up"))}
              >
                {stopping ? <Square className="size-3.5" /> : <Play className="size-3.5" />}
                {stopping ? "Stop" : "Start"}
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent className="max-w-sm">{primaryReason ?? `${primaryHint}. Runs on: ${cap.executionLocation}`}</TooltipContent>
        </Tooltip>
        <DropdownMenu>
          <Tooltip>
            <TooltipTrigger asChild>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" className="size-7" aria-label={`More actions for ${project.name}`}>
                  <EllipsisVertical className="size-3.5" />
                </Button>
              </DropdownMenuTrigger>
            </TooltipTrigger>
            <TooltipContent>More actions</TooltipContent>
          </Tooltip>
          <DropdownMenuContent align="end" className="w-60">
            <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">Compose CLI</DropdownMenuLabel>
            {VERBS.slice(1).map((v) => {
              const reason = reasonFor(v.verb);
              return (
                <DropdownMenuItem key={v.verb} disabled={!!reason} title={reason ?? v.hint} onSelect={() => onVerb(v.verb)}>
                  {v.icon} {v.label}
                </DropdownMenuItem>
              );
            })}
            {(["start", "stop"] as const).map((v) => {
              const reason = baseReason ?? (notCreated ? NOT_CREATED : undefined);
              return (
                <DropdownMenuItem key={v} disabled={!!reason} title={reason} onSelect={() => void verbs.run(v, project.name)}>
                  {v === "start" ? <Play /> : <Square />}
                  {v === "start" ? "Start" : "Stop"} project
                </DropdownMenuItem>
              );
            })}
            <DropdownMenuSeparator />
            <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">Engine operation (no Compose CLI)</DropdownMenuLabel>
            {(["start", "stop", "restart"] as const).map((a) => {
              const n = partitionByEligibility(a, containers).eligible.length;
              return (
                <DropdownMenuItem
                  key={a}
                  disabled={n === 0 || engine.pending}
                  title={n === 0 ? "No containers are eligible" : undefined}
                  onSelect={() => void engine.run(a, [...containers])}
                >
                  {a === "start" ? <Play /> : a === "stop" ? <Square /> : <RotateCw />}
                  {a === "start" ? "Start" : a === "stop" ? "Stop" : "Restart"} {n} container{n === 1 ? "" : "s"}
                </DropdownMenuItem>
              );
            })}
            {folderItems}
            {project.managed && (
              <>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={() => setConfirmForget(true)}>
                  <Trash2 /> Forget project…
                </DropdownMenuItem>
              </>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
        {dialogs}
      </div>
    );
  }

  return (
    <div className="flex flex-nowrap items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
      {VERBS.filter((v) => v.verb === "up" || v.verb === "down" || v.verb === "restart").map((v) => {
        const reason = baseReason ?? (notCreated && (v.verb === "down" || v.verb === "restart") ? NOT_CREATED : undefined);
        return (
          <Tooltip key={v.verb}>
            <TooltipTrigger asChild>
              <span className="inline-flex">
                <Button
                  variant="outline"
                  size={labels ? "sm" : "icon"}
                  className={labels ? "" : "size-7"}
                  disabled={!!reason}
                  aria-label={`Compose ${v.label} ${project.name}`}
                  onClick={() => onVerb(v.verb)}
                >
                  <span className="inline-flex [&_svg]:size-3.5">{v.icon}</span>
                  {labels && v.label}
                </Button>
              </span>
            </TooltipTrigger>
            <TooltipContent className="max-w-sm">{reason ?? `${v.hint}. Runs on: ${cap.executionLocation}`}</TooltipContent>
          </Tooltip>
        );
      })}
      <UpOptionsPopover project={project.name} labels={labels} disabledReason={baseReason} onUp={upOpts} />
      <WatchHeaderControl connectionId={connectionId} project={project} dialogOpen={watchOpen} onDialogOpenChange={setWatchOpen} />
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="outline"
            size="icon"
            className="size-7"
            aria-label={`More actions for ${project.name}`}
          >
            <EllipsisVertical className="size-3.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">Compose CLI</DropdownMenuLabel>
          <WatchMenuItem connectionId={connectionId} project={project} onChoose={() => setWatchOpen(true)} />
          {VERBS.filter((v) => v.verb === "pull" || v.verb === "build").map((v) => {
            const reason = baseReason;
            return (
              <DropdownMenuItem key={v.verb} disabled={!!reason} title={reason ?? v.hint} onSelect={() => onVerb(v.verb)}>
                {v.icon} {v.label}
              </DropdownMenuItem>
            );
          })}
          {(["start", "stop"] as const).map((v) => {
            const reason = baseReason ?? (notCreated ? NOT_CREATED : undefined);
            return (
              <DropdownMenuItem key={v} disabled={!!reason} title={reason} onSelect={() => void verbs.run(v, project.name)}>
                {v === "start" ? <Play /> : <Square />}
                {v === "start" ? "Start" : "Stop"} project
              </DropdownMenuItem>
            );
          })}
          {folderItems}
          {project.managed && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => setConfirmForget(true)}>
                <Trash2 /> Forget project…
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="outline"
            size={labels ? "sm" : "icon"}
            className={labels ? "" : "size-7"}
            disabled={engine.pending}
            aria-label={`Engine container actions for ${project.name}`}
          >
            <Layers className="size-3.5" />
            {labels && "Containers"}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">
            Engine operation on this project&apos;s containers (no Compose CLI)
          </DropdownMenuLabel>
          {(["start", "stop", "restart"] as const).map((a) => {
            const n = partitionByEligibility(a, containers).eligible.length;
            return (
              <DropdownMenuItem
                key={a}
                disabled={n === 0}
                title={n === 0 ? "No containers are eligible" : undefined}
                onSelect={() => void engine.run(a, [...containers])}
              >
                {a === "start" ? <Play /> : a === "stop" ? <Square /> : <RotateCw />}
                {a === "start" ? "Start" : a === "stop" ? "Stop" : "Restart"} {n} container{n === 1 ? "" : "s"}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>
      <ForgetProjectDialog
        connectionId={connectionId}
        project={project.name}
        files={project.configFiles ?? []}
        open={confirmForget}
        onOpenChange={setConfirmForget}
        onForgotten={onForgotten}
      />
      <DownDialog
        connectionId={connectionId}
        project={project.name}
        containers={containers}
        open={confirmDown}
        onOpenChange={setConfirmDown}
        onConfirm={(o) => {
          setConfirmDown(false);
          void verbs.run("down", project.name, { volumes: o.volumes });
        }}
      />
    </div>
  );
}
