import { Link } from "react-router-dom";
import { MoreHorizontal, Play, RotateCw, Square } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ContainerStatusBadge } from "@/components/containers/ContainerStatusBadge";
import { HealthBadge, StateDot } from "@/components/shared/StatusBadge";
import { formatBytes, formatDateTime, publishedPorts } from "@/components/containers/format";
import { formatAge } from "@/components/images/layers/layerFormat";
import { partitionByEligibility } from "@/lib/actionEligibility";
import type { ServiceRow as Row } from "@/lib/serviceRows";
import type { Container } from "@bindings/model";

export type Act = "start" | "stop" | "restart";
const ICON: Record<Act, React.ReactNode> = { start: <Play />, stop: <Square />, restart: <RotateCw /> };
const LABEL: Record<Act, string> = { start: "Start", stop: "Stop", restart: "Restart" };

export interface ServiceRowProps {
  row: Row;
  /** First row of its service among the rows shown: it carries the service-wide menu. */
  firstVisible: boolean;
  /** All containers of the row's service, for the service-wide menu on the first replica. */
  siblings: Container[];
  /** Image size in bytes; undefined when the image list has no match. */
  imageSize?: number;
  busy: boolean;
  /** Why Compose Up is disabled, if it is. */
  upReason?: string;
  /** False when Compose cannot run for this service: Up then shows only for a service without container, disabled. */
  composeAvailable: boolean;
  onAct: (a: Act, targets: Container[]) => void;
  onUp: () => void;
}

const dash = <span className="text-muted-foreground">—</span>;

function IconSlot({ act, container, busy, onAct }: { act: Act; container?: Container; busy: boolean; onAct: ServiceRowProps["onAct"] }) {
  if (!container || partitionByEligibility(act, [container]).eligible.length === 0) return <span className="size-7" aria-hidden />;
  const reason = busy ? "Another action is in progress" : undefined;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex">
          <Button variant="ghost" size="icon" className="size-7" disabled={!!reason} aria-label={`${LABEL[act]} ${container.name}`} onClick={() => onAct(act, [container])}>
            <span className="inline-flex [&_svg]:size-3.5">{ICON[act]}</span>
          </Button>
        </span>
      </TooltipTrigger>
      <TooltipContent>{reason ?? (act === "start" ? "Start this container" : LABEL[act])}</TooltipContent>
    </Tooltip>
  );
}

/** "Restart all 3" / "Stop all 3" / "Start all 3" for a service with several replicas. */
function ServiceMenu({ service, siblings, busy, onAct }: { service: string; siblings: Container[]; busy: boolean; onAct: ServiceRowProps["onAct"] }) {
  const items = (["restart", "stop", "start"] as const)
    .map((a) => ({ a, n: partitionByEligibility(a, siblings).eligible.length }))
    .filter((i) => i.n > 0);
  if (items.length === 0) return <span className="size-7" aria-hidden />;
  if (busy) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="inline-flex">
            <Button variant="ghost" size="icon" className="size-7" disabled aria-label={`All containers of service ${service}`}>
              <MoreHorizontal className="size-3.5" />
            </Button>
          </span>
        </TooltipTrigger>
        <TooltipContent>Another action is in progress</TooltipContent>
      </Tooltip>
    );
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="size-7" aria-label={`All containers of service ${service}`}>
          <MoreHorizontal className="size-3.5" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {items.map(({ a, n }) => (
          <DropdownMenuItem key={a} onSelect={() => onAct(a, siblings)}>
            {LABEL[a]} all {n}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function UpButton({ service, reason, onUp }: { service: string; reason?: string; onUp: () => void }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex">
          <Button variant="outline" size="sm" className="h-7 gap-1.5 px-2 text-xs" disabled={!!reason} aria-label={`Compose Up service ${service}`} onClick={onUp}>
            <Play className="size-3.5" /> Up
          </Button>
        </span>
      </TooltipTrigger>
      <TooltipContent className="max-w-sm">{reason ?? `Create and start this service (docker compose up -d ${service})`}</TooltipContent>
    </Tooltip>
  );
}

/** One table row per container (or per service that has none yet). */
export function ServiceRow({ row, firstVisible, siblings, imageSize, busy, upReason, composeAvailable, onAct, onUp }: ServiceRowProps) {
  const c = row.container;
  const ports = publishedPorts(c?.ports ?? []);
  // Up is shown when it would do something. Without Compose it stays only for a service that has no
  // container yet, disabled, so the reason can be read.
  const needsUp = !c || (!row.running && composeAvailable);
  return (
    <tr data-service={row.service} className="h-[42px] border-b border-border/60 hover:bg-accent/40">
      <td className="px-3">
        <div className="flex min-w-0 items-center gap-2">
          <StateDot state={c?.state ?? "none"} />
          <div className="min-w-0">
            <div className="flex items-baseline gap-1.5">
              <span className="truncate font-medium">{row.service || "—"}</span>
              {row.replicas > 1 && (
                <span className="whitespace-nowrap text-xs text-muted-foreground">
                  {row.replica} of {row.replicas}
                </span>
              )}
            </div>
            {c && (
              <Link to={`/containers/${c.id}`} className="block max-w-[260px] truncate text-[11px] leading-tight text-muted-foreground hover:underline" title={c.name}>
                {c.name}
              </Link>
            )}
          </div>
        </div>
      </td>
      <td className="px-3">
        {c ? (
          <span className="block max-w-[200px] truncate text-muted-foreground" title={c.image}>
            {c.image}
          </span>
        ) : (
          dash
        )}
      </td>
      <td className="px-3">
        {c ? (
          <ContainerStatusBadge container={c} />
        ) : (
          <Badge variant="outline" className="rounded-md border-dashed px-2 py-0.5 text-xs font-medium text-muted-foreground">
            Not created
          </Badge>
        )}
      </td>
      <td className="px-3">{c ? <HealthBadge health={c.health} /> : null}</td>
      <td className="px-3">
        {ports.length ? (
          <span className="font-mono text-xs" title={ports.join(", ")}>
            {ports.join(", ")}
          </span>
        ) : (
          dash
        )}
      </td>
      <td className="whitespace-nowrap px-3 tabular-nums">{imageSize == null ? dash : formatBytes(imageSize)}</td>
      <td className="whitespace-nowrap px-3 text-muted-foreground">
        {c?.created ? <span title={formatDateTime(new Date(c.created * 1000))}>{formatAge(c.created)}</span> : dash}
      </td>
      <td className="px-3">
        <div className="flex items-center justify-end gap-1">
          <div className="flex w-14 justify-end">{needsUp && <UpButton service={row.service} reason={upReason} onUp={onUp} />}</div>
          {/* Fixed slots keep the buttons of every row in the same columns: Up, Start, Stop, Restart, menu.
              Start, Stop and Restart are the container's own Engine actions, Compose or not. */}
          <IconSlot act="start" container={c} busy={busy} onAct={onAct} />
          <IconSlot act="stop" container={c} busy={busy} onAct={onAct} />
          <IconSlot act="restart" container={c} busy={busy} onAct={onAct} />
          {row.replicas > 1 && firstVisible ?<ServiceMenu service={row.service} siblings={siblings} busy={busy} onAct={onAct} /> : <span className="size-7" aria-hidden />}
        </div>
      </td>
    </tr>
  );
}
