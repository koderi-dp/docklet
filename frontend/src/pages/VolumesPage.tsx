import { Activity, Database, MoreHorizontal, Plus, RefreshCw, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { EmptyState } from "@/components/shared/EmptyState";
import { PageHeader } from "@/components/shared/PageHeader";
import { SelectionBar } from "@/components/shared/SelectionBar";
import { TableCard, TableToolbar } from "@/components/shared/TableCard";
import { CreateVolumeDialog } from "@/components/volumes/CreateVolumeDialog";
import { VolumeConfirmDialogs } from "@/components/volumes/VolumeConfirmDialogs";
import { VolumeInspector } from "@/components/volumes/VolumeInspector";
import { VolumesTable } from "@/components/volumes/VolumesTable";
import { UsageToolbar } from "@/components/volumes/parts";
import { useVolumesPage } from "@/components/volumes/useVolumesPage";
import { useEngineName } from "@/lib/engineKind";

export default function VolumesPage() {
  return <ConnectionGate>{({ connectionId }) => <VolumesView connectionId={connectionId} />}</ConnectionGate>;
}

function VolumesView({ connectionId }: { connectionId: string }) {
  const pg = useVolumesPage(connectionId);
  const { list } = pg;
  const engine = useEngineName(connectionId);

  return (
    <div className="relative flex h-full min-h-0">
      <div className="min-w-0 flex-1 overflow-auto p-5">
        <PageHeader
          title="Volumes"
          subtitle={`Persistent data stored on this ${engine} engine`}
          actions={
            <>
              <Button size="sm" onClick={() => pg.setCreateOpen(true)}>
                <Plus className="size-4" /> Create Volume
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button size="icon" variant="outline" className="size-8" aria-label="More options" title="More options">
                    <MoreHorizontal className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onSelect={() => void list.refetch()}><RefreshCw /> Refresh</DropdownMenuItem>
                  <DropdownMenuItem variant="destructive" disabled={pg.pruning} onSelect={() => { pg.setUnderstood(false); pg.setPruneOpen(true); }}>
                    <Trash2 /> Prune unused volumes…
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          }
        />

        {list.isLoading ? (
          <Skeleton className="h-72 w-full rounded-[10px]" />
        ) : list.error ? (
          <EmptyState icon={<Activity />} title="Could not load volumes">
            <p className="text-destructive">{String(list.error)}</p>
            <Button className="mt-3" onClick={() => void list.refetch()}>Retry</Button>
          </EmptyState>
        ) : (
          <TableCard>
            <TableToolbar
              selection={
                pg.selectedVolumes.length > 0 ? (
                  <SelectionBar count={pg.selectedVolumes.length} onClear={() => pg.setSelected(new Set())}>
                    <Button
                      size="sm"
                      variant="outline"
                      className="border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive"
                      disabled={pg.removing}
                      onClick={() => pg.setConfirmRemove(pg.selectedVolumes)}
                    >
                      <Trash2 className="size-3.5" /> Remove
                    </Button>
                  </SelectionBar>
                ) : undefined
              }
            >
              <UsageToolbar search={pg.search} onSearch={pg.setSearch} filter={pg.filter} onFilter={pg.setFilter} counts={pg.counts} placeholder="Search volumes, drivers, labels" />
            </TableToolbar>
            {pg.volumes.length === 0 ? (
              <EmptyState icon={<Database />} title="No volumes on this Engine">
                <p>Create a volume, or let containers create them. Volumes are listed here as the Engine reports them.</p>
                <Button className="mt-3" onClick={() => pg.setCreateOpen(true)}>Create Volume</Button>
              </EmptyState>
            ) : pg.visible.length === 0 ? (
              <EmptyState icon={<Database />} title="No matching volumes">
                <Button variant="outline" onClick={() => { pg.setSearch(""); pg.setFilter("all"); }}>Clear filters</Button>
              </EmptyState>
            ) : (
              <VolumesTable
                rows={pg.visible}
                selected={pg.selected}
                inspected={pg.inspected}
                allVisibleSelected={pg.allVisibleSelected}
                onToggleAll={pg.toggleAllVisible}
                onToggle={pg.toggle}
                onInspect={pg.setInspected}
                onRemove={(v) => pg.setConfirmRemove([v])}
              />
            )}
          </TableCard>
        )}
      </div>

      {pg.inspectedVolume && (
        <VolumeInspector
          volume={pg.inspectedVolume}
          busy={pg.removing}
          onClose={() => pg.setInspected(null)}
          onRemove={(v) => pg.setConfirmRemove([v])}
        />
      )}

      <CreateVolumeDialog connectionId={connectionId} open={pg.createOpen} onOpenChange={pg.setCreateOpen} />
      <VolumeConfirmDialogs
        confirmRemove={pg.confirmRemove}
        onCancelRemove={() => pg.setConfirmRemove(null)}
        onRemove={(t) => void pg.doRemove(t)}
        pruneOpen={pg.pruneOpen}
        onPruneOpenChange={pg.setPruneOpen}
        unusedVolumes={pg.unusedVolumes}
        understood={pg.understood}
        onUnderstood={pg.setUnderstood}
        onPrune={() => void pg.startPrune()}
      />
    </div>
  );
}
