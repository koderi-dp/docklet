import { useMemo } from "react";
import { Link } from "react-router-dom";
import {
  columnVisibilityFeature,
  createColumnHelper,
  tableFeatures,
  useTable,
  type OnChangeFn,
  type ColumnVisibilityState,
} from "@tanstack/react-table";
import type { Container, ContainerStat } from "@bindings/model";
import { Badge } from "@/components/ui/badge";
import { Loader2 } from "lucide-react";
import { HealthBadge, StateDot } from "@/components/shared/StatusBadge";
import type { ContainerAction } from "@/lib/actionEligibility";
import { checkState, type ComposeGroup, type FlatRow } from "@/lib/composeGrouping";
import { cn } from "@/lib/utils";
import { SortTh } from "@/components/tables/SortTh";
import { ComposeGroupRow } from "./ComposeGroupRow";
import { ContainerActionMenu } from "./ContainerActionMenu";
import { CpuCell, MemoryCell, PortsCell, UptimeCell } from "./ContainerCells";
import { TriCheckbox } from "./TriCheckbox";
import { ContainerStatusBadge } from "./ContainerStatusBadge";

// Secondary columns drop out as the table's own width shrinks (inspector open / narrow window).
const RESPONSIVE_COL: Record<string, string> = {
  project: "hidden @[900px]:table-cell",
  cpu: "hidden text-right @[900px]:table-cell",
  memory: "hidden text-right @[820px]:table-cell",
  uptime: "hidden pl-6 @[1000px]:table-cell",
};

/** Columns the user can sort by (see lib/containerSort). */
const SORTABLE = new Set(["name", "image", "project", "status", "health", "cpu", "memory", "uptime"]);

const features = tableFeatures({ columnVisibilityFeature });
const helper = createColumnHelper<typeof features, FlatRow>();

export const TOGGLEABLE_COLUMNS = [
  { id: "image", label: "Image" },
  { id: "health", label: "Health" },
  { id: "ports", label: "Ports" },
  { id: "cpu", label: "CPU" },
  { id: "memory", label: "Memory" },
  { id: "uptime", label: "Uptime" },
] as const;

export interface ContainerTableProps {
  rows: FlatRow[];
  mode: "grouped" | "flat";
  selected: ReadonlySet<string>;
  inspectedId: string | null;
  inFlight: ReadonlySet<string>;
  stats: ReadonlyMap<string, ContainerStat>;
  isCollapsed: (g: ComposeGroup) => boolean;
  canRunCompose: boolean;
  composeReason?: string;
  columnVisibility: ColumnVisibilityState;
  onColumnVisibilityChange: OnChangeFn<ColumnVisibilityState>;
  onToggleSelect: (id: string) => void;
  onToggleSelectMany: (ids: string[]) => void;
  /** Shift+click: set every container between the last-toggled one and `id` (display order). */
  onSelectRange: (orderedIds: string[], id: string) => void;
  onOpenDetails: (c: Container) => void;
  onInspect: (c: Container) => void;
  onToggleGroup: (g: ComposeGroup) => void;
  onExpandAll: () => void;
  onCollapseAll: () => void;
  onContainerAction: (action: ContainerAction, c: Container) => void;
  onGroupAction: (action: ContainerAction, g: ComposeGroup) => void;
  onCopyId: (c: Container) => void;
}

export function ContainerTable(p: ContainerTableProps) {
  const containerRows = useMemo(
    () => p.rows.filter((r): r is Extract<FlatRow, { kind: "container" }> => r.kind === "container"),
    [p.rows],
  );
  const allIds = useMemo(() => containerRows.map((r) => r.container.id), [containerRows]);
  const headerState = checkState(allIds, p.selected);

  const columns = useMemo(
    () =>
      helper.columns([
        helper.display({
          id: "select",
          header: () => (
            <TriCheckbox
              state={headerState}
              onToggle={() => p.onToggleSelectMany(allIds)}
              label="Select all visible containers"
            />
          ),
          cell: (ctx) => {
            const r = ctx.row.original;
            if (r.kind !== "container") return null;
            return (
              <TriCheckbox
                state={p.selected.has(r.container.id) ? "checked" : "unchecked"}
                onToggle={(shift) => (shift ? p.onSelectRange(allIds, r.container.id) : p.onToggleSelect(r.container.id))}
                label={`Select ${r.container.name}`}
              />
            );
          },
        }),
        helper.display({
          id: "name",
          header: "Name",
          cell: (ctx) => {
            const r = ctx.row.original;
            if (r.kind !== "container") return null;
            const c = r.container;
            return (
              <div className="flex min-w-0 items-center gap-2">
                {p.inFlight.has(c.id) ? (
                  <Loader2 className="size-3.5 shrink-0 animate-spin text-warning" aria-label="Action in progress" />
                ) : (
                  <StateDot state={c.state} />
                )}
                <span className="truncate font-medium" title={c.name}>
                  {c.name}
                </span>
              </div>
            );
          },
        }),
        helper.display({
          id: "image",
          header: "Image",
          cell: (ctx) => {
            const r = ctx.row.original;
            if (r.kind !== "container") return null;
            return (
              <span className="block max-w-[160px] truncate text-muted-foreground" title={r.container.image}>
                {r.container.image}
              </span>
            );
          },
        }),
        helper.display({
          id: "project",
          header: "Project",
          cell: (ctx) => {
            const r = ctx.row.original;
            if (r.kind !== "container") return null;
            const proj = r.container.composeProject;
            return proj ? (
              <Link to={`/compose/${encodeURIComponent(proj)}`} onClick={(e) => e.stopPropagation()}>
                <Badge variant="secondary" className="max-w-[140px] truncate font-normal hover:bg-accent">
                  {proj}
                </Badge>
              </Link>
            ) : (
              <span className="text-muted-foreground">Standalone</span>
            );
          },
        }),
        helper.display({
          id: "status",
          header: "Status",
          cell: (ctx) => {
            const r = ctx.row.original;
            return r.kind === "container" ? <ContainerStatusBadge container={r.container} /> : null;
          },
        }),
        helper.display({
          id: "health",
          header: "Health",
          cell: (ctx) => {
            const r = ctx.row.original;
            return r.kind === "container" ? <HealthBadge health={r.container.health} /> : null;
          },
        }),
        helper.display({
          id: "ports",
          header: "Ports",
          cell: (ctx) => {
            const r = ctx.row.original;
            return r.kind === "container" ? <PortsCell container={r.container} /> : null;
          },
        }),
        helper.display({
          id: "cpu",
          header: "CPU",
          cell: (ctx) => {
            const r = ctx.row.original;
            return r.kind === "container" ? <CpuCell container={r.container} stat={p.stats.get(r.container.id)} /> : null;
          },
        }),
        helper.display({
          id: "memory",
          header: "Memory",
          cell: (ctx) => {
            const r = ctx.row.original;
            return r.kind === "container" ? <MemoryCell container={r.container} stat={p.stats.get(r.container.id)} /> : null;
          },
        }),
        helper.display({
          id: "uptime",
          header: "Uptime",
          cell: (ctx) => {
            const r = ctx.row.original;
            return r.kind === "container" ? <UptimeCell container={r.container} /> : null;
          },
        }),
        helper.display({
          id: "actions",
          header: () => <span className="sr-only">Actions</span>,
          cell: (ctx) => {
            const r = ctx.row.original;
            if (r.kind !== "container") return null;
            const c = r.container;
            return (
              <div className="flex justify-end">
                <ContainerActionMenu
                  id={c.id}
                  name={c.name}
                  state={c.state}
                  busy={p.inFlight.has(c.id)}
                  onAction={(a) => p.onContainerAction(a, c)}
                  onInspect={() => p.onInspect(c)}
                  onOpenDetails={() => p.onOpenDetails(c)}
                  onCopyId={() => p.onCopyId(c)}
                />
              </div>
            );
          },
        }),
      ]),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [p.selected, p.inFlight, p.stats, headerState, allIds, p.onToggleSelect, p.onToggleSelectMany, p.onSelectRange, p.onInspect, p.onContainerAction, p.onCopyId],
  );

  const table = useTable({
    features,
    columns,
    data: p.rows,
    getRowId: (r) => r.id,
    state: { columnVisibility: p.columnVisibility },
    onColumnVisibilityChange: p.onColumnVisibilityChange,
  });

  const visibleColCount = table.getVisibleLeafColumns().length;

  return (
    <div className="@container overflow-x-auto">
      <table className="w-full min-w-[560px] border-collapse text-[13px]">
        <thead>
          {table.getHeaderGroups().map((hg) => (
            <tr key={hg.id} className="border-b border-border text-left text-xs font-medium text-muted-foreground">
              {hg.headers.map((h) =>
                SORTABLE.has(h.column.id) && typeof h.column.columnDef.header === "string" ? (
                  <SortTh key={h.id} table="containers" id={h.column.id} className={RESPONSIVE_COL[h.column.id]}>
                    {h.column.columnDef.header}
                  </SortTh>
                ) : (
                  <th
                    key={h.id}
                    scope="col"
                    className={cn("h-10 whitespace-nowrap px-3 font-medium", h.column.id === "select" && "w-10", RESPONSIVE_COL[h.column.id])}
                  >
                    {h.isPlaceholder ? null : <table.FlexRender header={h} />}
                  </th>
                ),
              )}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row) => {
            const r = row.original;
            if (r.kind === "group") {
              const g = r.group;
              const ids = g.visible.map((c) => c.id);
              return (
                <tr key={row.id}>
                  <td colSpan={visibleColCount} className="p-0">
                    <ComposeGroupRow
                      group={g}
                      collapsed={p.isCollapsed(g)}
                      checkState={checkState(ids, p.selected)}
                      busy={g.visible.some((c) => p.inFlight.has(c.id))}
                      canRunCompose={p.canRunCompose}
                      composeReason={p.composeReason}
                      onToggleCollapsed={() => p.onToggleGroup(g)}
                      onToggleSelect={() => p.onToggleSelectMany(ids)}
                      onGroupAction={(a) => p.onGroupAction(a, g)}
                      onExpandAll={p.onExpandAll}
                      onCollapseAll={p.onCollapseAll}
                    />
                  </td>
                </tr>
              );
            }
            const c = r.container;
            const isChecked = p.selected.has(c.id);
            return (
              <tr
                key={row.id}
                tabIndex={0}
                aria-selected={p.inspectedId === c.id}
                data-state={isChecked ? "selected" : undefined}
                onClick={() => p.onInspect(c)}
                onDoubleClick={() => p.onOpenDetails(c)}
                onKeyDown={(e) => {
                  if (e.target !== e.currentTarget) return;
                  if (e.key === "Enter") {
                    e.preventDefault();
                    p.onOpenDetails(c);
                  } else if (e.key === " ") {
                    e.preventDefault();
                    p.onInspect(c);
                  }
                }}
                className={cn(
                  "h-[42px] cursor-pointer border-b border-border/60 outline-none transition-colors hover:bg-muted/50 focus-visible:bg-muted/60 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/60",
                  isChecked && "bg-primary/[0.07] hover:bg-primary/10",
                  p.inspectedId === c.id && "bg-accent hover:bg-accent [&>td:first-child]:shadow-[inset_2px_0_0_0_var(--primary)]",
                )}
              >
                {row.getVisibleCells().map((cell) => (
                  <td
                    key={cell.id}
                    className={cn("whitespace-nowrap px-3", cell.column.id === "select" && p.mode === "grouped" && "pl-9", RESPONSIVE_COL[cell.column.id])} // ~24px child indent
                  >
                    <table.FlexRender cell={cell} />
                  </td>
                ))}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
