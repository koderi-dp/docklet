import { useMemo, useRef } from "react";
import {
  columnVisibilityFeature,
  createColumnHelper,
  tableFeatures,
  useTable,
  type OnChangeFn,
  type ColumnVisibilityState,
} from "@tanstack/react-table";
import type { Container, ContainerStat } from "@bindings/model";
import type { ContainerAction } from "@/lib/actionEligibility";
import { checkState, type ComposeGroup, type FlatRow } from "@/lib/composeGrouping";
import { cn } from "@/lib/utils";
import { SortTh } from "@/components/tables/SortTh";
import { useProjectTotals } from "@/hooks/useProjectTotals";
import { ComposeGroupRow, type GroupHandlers } from "./ComposeGroupRow";
import { ContainerRow, RESPONSIVE_COL, type RowHandlers } from "./ContainerRow";
import { TriCheckbox } from "./TriCheckbox";
import { useGroupColumns } from "./useGroupColumns";

/** Columns the user can sort by (see lib/containerSort). */
const SORTABLE = new Set(["name", "image", "project", "status", "health", "cpu", "memory", "uptime"]);

const features = tableFeatures({ columnVisibilityFeature });
const helper = createColumnHelper<typeof features, FlatRow>();

// The tanstack table only drives the header and column visibility; rows are rendered by ContainerRow.
const NO_ROWS: FlatRow[] = [];

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

  const live = useRef({ p, headerState, allIds });
  live.current = { p, headerState, allIds };

  const columns = useMemo(
    () =>
      helper.columns([
        helper.display({
          id: "select",
          header: () => (
            <TriCheckbox
              state={live.current.headerState}
              onToggle={() => live.current.p.onToggleSelectMany(live.current.allIds)}
              label="Select all visible containers"
            />
          ),
        }),
        helper.display({ id: "name", header: "Name" }),
        helper.display({ id: "image", header: "Image" }),
        helper.display({ id: "project", header: "Project" }),
        helper.display({ id: "status", header: "Status" }),
        helper.display({ id: "health", header: "Health" }),
        helper.display({ id: "ports", header: "Ports" }),
        helper.display({ id: "cpu", header: "CPU" }),
        helper.display({ id: "memory", header: "Memory" }),
        helper.display({ id: "uptime", header: "Uptime" }),
        helper.display({ id: "actions", header: () => <span className="sr-only">Actions</span> }),
      ]),
    // The header reads the latest props through `live`, so the columns stay stable across renders.
    [],
  );

  const table = useTable({
    features,
    columns,
    data: NO_ROWS,
    state: { columnVisibility: p.columnVisibility },
    onColumnVisibilityChange: p.onColumnVisibilityChange,
  });

  const colKey = table
    .getVisibleLeafColumns()
    .map((c) => c.id)
    .join(",");
  const visibleColumns = useMemo(() => colKey.split(","), [colKey]);

  // Created once, reading the latest props at call time, so memoised rows never see a new handler.
  const rowHandlers = useMemo<RowHandlers>(
    () => ({
      onToggleSelect: (id, shift) =>
        shift ? live.current.p.onSelectRange(live.current.allIds, id) : live.current.p.onToggleSelect(id),
      onInspect: (c) => live.current.p.onInspect(c),
      onOpenDetails: (c) => live.current.p.onOpenDetails(c),
      onAction: (a, c) => live.current.p.onContainerAction(a, c),
      onCopyId: (c) => live.current.p.onCopyId(c),
    }),
    [],
  );
  const groupHandlers = useMemo<GroupHandlers>(
    () => ({
      onToggleCollapsed: (g) => live.current.p.onToggleGroup(g),
      onToggleSelect: (g) => live.current.p.onToggleSelectMany(g.visible.map((c) => c.id)),
      onGroupAction: (a, g) => live.current.p.onGroupAction(a, g),
      onExpandAll: () => live.current.p.onExpandAll(),
      onCollapseAll: () => live.current.p.onCollapseAll(),
    }),
    [],
  );

  const { ref: wrapRef, cols: groupCols } = useGroupColumns(p.columnVisibility);
  // Group rows sum the stats that already stream for the rows; memoised on the (cadence-limited) stats object.
  const groupContainers = useMemo(() => p.rows.flatMap((r) => (r.kind === "group" ? r.group.all : [])), [p.rows]);
  const projectLive = useProjectTotals(p.rows.find((r) => r.kind === "group")?.group.connectionId ?? "", groupContainers, p.stats);

  return (
    <div ref={wrapRef} className="@container overflow-x-auto">
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
          {p.rows.map((r) => {
            if (r.kind === "group") {
              const g = r.group;
              return (
                <ComposeGroupRow
                  key={r.id}
                  group={g}
                  cols={groupCols}
                  totals={g.project ? projectLive.totals.get(g.project) : undefined}
                  idle={g.project ? projectLive.idle.get(g.project) : undefined}
                  collapsed={p.isCollapsed(g)}
                  checkState={checkState(
                    g.visible.map((c) => c.id),
                    p.selected,
                  )}
                  busy={g.visible.some((c) => p.inFlight.has(c.id))}
                  canRunCompose={p.canRunCompose}
                  composeReason={p.composeReason}
                  handlers={groupHandlers}
                />
              );
            }
            const c = r.container;
            return (
              <ContainerRow
                key={r.id}
                container={c}
                stat={p.stats.get(c.id)}
                checked={p.selected.has(c.id)}
                inspected={p.inspectedId === c.id}
                busy={p.inFlight.has(c.id)}
                grouped={p.mode === "grouped"}
                columns={visibleColumns}
                handlers={rowHandlers}
              />
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
