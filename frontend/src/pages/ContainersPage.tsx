import { useNavigate } from "react-router-dom";
import { Activity, Box, MoreHorizontal, Play, Plus, RotateCw, Square, SquareStack, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { ConnectionState } from "@bindings/model";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { PageHeader } from "@/components/shared/PageHeader";
import { SelectionBar } from "@/components/shared/SelectionBar";
import { TableCard, TableToolbar } from "@/components/shared/TableCard";
import { ContainerTable, TOGGLEABLE_COLUMNS } from "@/components/containers/ContainerTable";
import { ContainersToolbar } from "@/components/containers/ContainersToolbar";
import { ContainerInspector } from "@/components/containers/ContainerInspector";
import { ContainerSummaryCards } from "@/components/containers/ContainerSummaryCards";
import { ContainerConfirmDialogs } from "@/components/containers/ContainerConfirmDialogs";
import CreateContainerDialog from "@/components/containers/CreateContainerDialog";
import { RecreateHost } from "@/components/containers/recreate/RecreateHost";
import { useContainersPage } from "@/components/containers/useContainersPage";
import { useActiveConnection, useSelectConnection } from "@/hooks/useActiveConnection";
import { type ContainerAction } from "@/lib/actionEligibility";
import { EMPTY_FILTERS, hasActiveFilters } from "@/lib/composeGrouping";

function Message({ icon, title, children }: { icon: React.ReactNode; title: string; children?: React.ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-2 px-6 py-16 text-center text-muted-foreground">
      <div className="[&_svg]:size-8">{icon}</div>
      <h2 className="text-base font-semibold text-foreground">{title}</h2>
      <div className="max-w-md text-sm">{children}</div>
    </div>
  );
}

export default function ContainersPage() {
  const { connectionId, status, profile, isConnected } = useActiveConnection();
  const navigate = useNavigate();
  const select = useSelectConnection();

  if (!connectionId)
    return (
      <Message icon={<Box />} title="Connect to a Docker Engine">
        <p>Select an engine from the sidebar to see its containers.</p>
        <Button className="mt-3" onClick={() => navigate("/connections")}>Manage connections</Button>
      </Message>
    );
  if (status?.state === ConnectionState.StateConnecting)
    return (
      <div className="space-y-4 p-5">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-20 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  if (!isConnected)
    return (
      <Message icon={<Activity />} title={`${profile?.name ?? "Engine"} is not connected`}>
        <p>{status?.error ?? "Connect to load its containers."}</p>
        {profile && <p className="mt-1 font-mono text-xs">{profile.endpoint}</p>}
        <div className="mt-3 flex justify-center gap-2">
          <Button onClick={() => select(connectionId)}>Reconnect</Button>
          <Button variant="outline" onClick={() => navigate("/connections")}>Connections</Button>
        </div>
        {status?.diagnostics && (
          <pre className="mt-3 max-h-40 overflow-auto rounded-md bg-muted/40 p-2 text-left font-mono text-[11px]">{status.diagnostics}</pre>
        )}
      </Message>
    );

  // key => all page state (selection, inspector, filters) resets on engine switch.
  return (
    <ContainersView
      key={connectionId}
      connectionId={connectionId}
      canRunCompose={!!status?.capabilities.canRunCompose}
      composeReason={status?.capabilities.composeReason}
    />
  );
}

interface ViewProps {
  connectionId: string;
  canRunCompose: boolean;
  composeReason?: string;
}

function ContainersView({ connectionId, canRunCompose, composeReason }: ViewProps) {
  const pg = useContainersPage(connectionId);
  const { list } = pg;

  const bulkBtn = (action: ContainerAction, icon: React.ReactNode, label: string, destructive = false) => (
    <Button
      size="sm"
      variant="outline"
      className={destructive ? "h-8 border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive" : "h-8"}
      disabled={pg.mutation.isPending}
      onClick={() => pg.bulk(action)}
    >
      {icon}
      {label}
    </Button>
  );

  return (
    <div className="relative flex h-full min-h-0">
      <div className="min-w-0 flex-1 overflow-auto p-5">
        <PageHeader
          title="Containers"
          subtitle="Manage running and stopped containers"
          actions={
            <>
              <Button size="sm" onClick={() => pg.setCreateOpen(true)}>
                <Plus className="size-4" /> New Container
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button size="icon" variant="outline" className="size-8" aria-label="More options" title="More options">
                    <MoreHorizontal className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onSelect={() => void list.refetch()}>Refresh</DropdownMenuItem>
                  {pg.view === "grouped" && (
                    <>
                      <DropdownMenuItem onSelect={() => pg.setAllGroups(pg.allGroupKeys, false)}>Expand all</DropdownMenuItem>
                      <DropdownMenuItem onSelect={() => pg.setAllGroups(pg.allGroupKeys, true)}>Collapse all</DropdownMenuItem>
                    </>
                  )}
                  <DropdownMenuSeparator />
                  <DropdownMenuLabel>Columns</DropdownMenuLabel>
                  {TOGGLEABLE_COLUMNS.map((col) => (
                    <DropdownMenuCheckboxItem
                      key={col.id}
                      checked={pg.colVis[col.id] !== false}
                      onCheckedChange={(v) => pg.setColVis((s) => ({ ...s, [col.id]: !!v }))}
                      onSelect={(e) => e.preventDefault()}
                    >
                      {col.label}
                    </DropdownMenuCheckboxItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          }
        />

        {list.isLoading ? (
          <div>
            <div className="mb-4 grid grid-cols-4 gap-3">
              {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-20 rounded-[10px]" />)}
            </div>
            <Skeleton className="h-72 w-full rounded-[10px]" />
          </div>
        ) : list.error ? (
          <Message icon={<Activity />} title="Could not load containers">
            <p className="text-destructive">{String(list.error)}</p>
            <Button className="mt-3" onClick={() => void list.refetch()}>Retry</Button>
          </Message>
        ) : (
          <>
            <ContainerSummaryCards summary={pg.summary} />

            <TableCard>
              <TableToolbar
                selection={
                  pg.selected.size > 0 ? (
                    <SelectionBar
                      count={pg.selected.size}
                      onClear={() => pg.setSelected(new Set())}
                      note={pg.hiddenSelected > 0 ? `${pg.hiddenSelected} hidden by the current filters, still included in bulk actions` : undefined}
                    >
                      {bulkBtn("start", <Play className="size-3.5" />, "Start")}
                      {bulkBtn("stop", <Square className="size-3.5" />, "Stop")}
                      {bulkBtn("restart", <RotateCw className="size-3.5" />, "Restart")}
                      {bulkBtn("remove", <Trash2 className="size-3.5" />, "Delete", true)}
                    </SelectionBar>
                  ) : undefined
                }
              >
                <ContainersToolbar filters={pg.filters} onChange={pg.setFilters} counts={pg.tabCounts} view={pg.view} onViewChange={pg.setView} />
              </TableToolbar>
              {pg.containers.length === 0 ? (
                <Message icon={<SquareStack />} title="No containers on this Engine">
                  <p>Create one to get started.</p>
                  <Button className="mt-3" onClick={() => pg.setCreateOpen(true)}>
                    <Plus className="size-4" /> Create container
                  </Button>
                </Message>
              ) : pg.visibleContainers.length === 0 ? (
                <Message icon={<SquareStack />} title="No matching containers">
                  <Button variant="outline" onClick={() => pg.setFilters(EMPTY_FILTERS)} disabled={!hasActiveFilters(pg.filters)}>
                    Clear filters
                  </Button>
                </Message>
              ) : (
                <ContainerTable
                  rows={pg.rows}
                  mode={pg.view}
                  selected={pg.selected}
                  inspectedId={pg.inspectedId}
                  inFlight={pg.inFlight}
                  stats={pg.stats}
                  isCollapsed={pg.isCollapsed}
                  canRunCompose={canRunCompose}
                  composeReason={composeReason}
                  columnVisibility={pg.colVis}
                  onColumnVisibilityChange={(u) => pg.setColVis((s) => (typeof u === "function" ? u(s) : u))}
                  onToggleSelect={pg.onToggleSelect}
                  onToggleSelectMany={pg.onToggleSelectMany}
                  onSelectRange={pg.onSelectRange}
                  onOpenDetails={pg.onOpenDetails}
                  onInspect={pg.onInspect}
                  onToggleGroup={(g) => pg.toggleGroup(g.collapseKey)}
                  onExpandAll={() => pg.setAllGroups(pg.allGroupKeys, false)}
                  onCollapseAll={() => pg.setAllGroups(pg.allGroupKeys, true)}
                  onContainerAction={(a, c) => pg.requestAction(a, [c])}
                  onGroupAction={(a, g) => pg.requestAction(a, g.visible)}
                  onCopyId={(c) => void navigator.clipboard.writeText(c.id).then(() => toast.success("Container ID copied"))}
                />
              )}
            </TableCard>
          </>
        )}
      </div>

      {pg.inspected && (
        <ContainerInspector
          connectionId={connectionId}
          container={pg.inspected}
          stat={pg.stats.get(pg.inspected.id)}
          busy={pg.inFlight.has(pg.inspected.id)}
          width={pg.inspectorWidth}
          onResize={pg.setInspectorWidth}
          onClose={() => pg.setInspectedId(null)}
          onAction={(a, c) => pg.requestAction(a, [c])}
        />
      )}

      <CreateContainerDialog connectionId={connectionId} open={pg.createOpen} onOpenChange={pg.setCreateOpen} />
      <RecreateHost />
      <ContainerConfirmDialogs
        confirmKill={pg.confirmKill}
        onCancelKill={() => pg.setConfirmKill(null)}
        onKill={(t) => void pg.run("kill", t)}
        confirmRemove={pg.confirmRemove}
        onCancelRemove={() => pg.setConfirmRemove(null)}
        onRemove={(t) => void pg.run("remove", t, pg.forceNeeded)}
        forceNeeded={pg.forceNeeded}
      />
    </div>
  );
}
