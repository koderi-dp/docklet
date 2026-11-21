import { memo, type ReactNode } from "react";
import { Link } from "react-router-dom";
import type { Container, ContainerStat } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { Loader2 } from "lucide-react";
import { HealthBadge, StateDot } from "@/components/shared/StatusBadge";
import type { ContainerAction } from "@/lib/actionEligibility";
import { cn } from "@/lib/utils";
import { ContainerActionMenu } from "./ContainerActionMenu";
import { CpuCell, MemoryCell, PortsCell, UptimeCell } from "./ContainerCells";
import { TriCheckbox } from "./TriCheckbox";
import { ContainerStatusBadge } from "./ContainerStatusBadge";

// Secondary columns drop out as the table's own width shrinks (inspector open / narrow window).
export const RESPONSIVE_COL: Record<string, string> = {
  project: "hidden @[900px]:table-cell",
  cpu: "hidden text-right @[900px]:table-cell",
  memory: "hidden text-right @[820px]:table-cell",
  uptime: "hidden pl-6 @[1200px]:table-cell",
};

export interface RowHandlers {
  onToggleSelect(id: string, shift: boolean): void;
  onInspect(c: Container): void;
  onOpenDetails(c: Container): void;
  onAction(a: ContainerAction, c: Container): void;
  onCopyId(c: Container): void;
}

export interface ContainerRowProps {
  container: Container;
  stat: ContainerStat | undefined;
  checked: boolean;
  inspected: boolean;
  busy: boolean;
  grouped: boolean;
  columns: readonly string[];
  handlers: RowHandlers;
}

function renderCell(id: string, p: ContainerRowProps): ReactNode {
  const c = p.container;
  switch (id) {
    case "select":
      return (
        <TriCheckbox
          state={p.checked ? "checked" : "unchecked"}
          onToggle={(shift) => p.handlers.onToggleSelect(c.id, shift)}
          label={`Select ${c.name}`}
        />
      );
    case "name":
      return (
        <div className="flex min-w-0 max-w-[260px] items-center gap-2">
          {p.busy ? (
            <Loader2 className="size-3.5 shrink-0 animate-spin text-warning" aria-label="Action in progress" />
          ) : (
            <StateDot state={c.state} />
          )}
          <span className="truncate font-medium" title={c.name}>
            {c.name}
          </span>
        </div>
      );
    case "image":
      return (
        <span className="block max-w-[160px] truncate text-muted-foreground" title={c.image}>
          {c.image}
        </span>
      );
    case "project": {
      const proj = c.composeProject;
      return proj ? (
        <Link to={`/compose/${encodeURIComponent(proj)}`} onClick={(e) => e.stopPropagation()}>
          <Badge variant="secondary" className="max-w-[140px] truncate font-normal hover:bg-accent">
            {proj}
          </Badge>
        </Link>
      ) : (
        <span className="text-muted-foreground">Standalone</span>
      );
    }
    case "status":
      return <ContainerStatusBadge container={c} />;
    case "health":
      return <HealthBadge health={c.health} />;
    case "ports":
      return <PortsCell container={c} />;
    case "cpu":
      return <CpuCell container={c} stat={p.stat} />;
    case "memory":
      return <MemoryCell container={c} stat={p.stat} />;
    case "uptime":
      return <UptimeCell container={c} />;
    case "actions":
      return (
        <div className="flex justify-end">
          <ContainerActionMenu
            id={c.id}
            name={c.name}
            state={c.state}
            busy={p.busy}
            onAction={(a) => p.handlers.onAction(a, c)}
            onInspect={() => p.handlers.onInspect(c)}
            onOpenDetails={() => p.handlers.onOpenDetails(c)}
            onCopyId={() => p.handlers.onCopyId(c)}
            projectDir={c.composeWorkingDir}
          />
        </div>
      );
    default:
      return null;
  }
}

export const ContainerRow = memo(function ContainerRow(p: ContainerRowProps) {
  const c = p.container;
  return (
    <tr
      tabIndex={0}
      aria-selected={p.inspected}
      data-state={p.checked ? "selected" : undefined}
      onClick={() => p.handlers.onInspect(c)}
      onDoubleClick={() => p.handlers.onOpenDetails(c)}
      onKeyDown={(e) => {
        if (e.target !== e.currentTarget) return;
        if (e.key === "Enter") {
          e.preventDefault();
          p.handlers.onOpenDetails(c);
        } else if (e.key === " ") {
          e.preventDefault();
          p.handlers.onInspect(c);
        }
      }}
      className={cn(
        "h-[42px] cursor-pointer border-b border-border/60 outline-none transition-colors hover:bg-muted/50 focus-visible:bg-muted/60 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/60",
        p.checked && "bg-primary/[0.07] hover:bg-primary/10",
        p.inspected && "bg-accent hover:bg-accent [&>td:first-child]:shadow-[inset_2px_0_0_0_var(--primary)]",
      )}
    >
      {p.columns.map((id) => (
        <td
          key={id}
          className={cn("whitespace-nowrap px-3", id === "select" && p.grouped && "pl-9", RESPONSIVE_COL[id])} // ~24px child indent
        >
          {renderCell(id, p)}
        </td>
      ))}
    </tr>
  );
});
