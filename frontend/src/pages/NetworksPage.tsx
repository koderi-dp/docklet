import { Activity, MoreHorizontal, Network as NetworkIcon, Plus, RefreshCw, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { EmptyState } from "@/components/shared/EmptyState";
import { PageHeader } from "@/components/shared/PageHeader";
import { SelectionBar } from "@/components/shared/SelectionBar";
import { TableCard, TableToolbar } from "@/components/shared/TableCard";
import { CreateNetworkDialog } from "@/components/networks/CreateNetworkDialog";
import { NetworkConfirmDialogs } from "@/components/networks/NetworkConfirmDialogs";
import { NetworkInspector } from "@/components/networks/NetworkInspector";
import { NetworksTable } from "@/components/networks/NetworksTable";
import { useNetworksPage } from "@/components/networks/useNetworksPage";
import { UsageToolbar } from "@/components/volumes/parts";

export default function NetworksPage() {
  return <ConnectionGate>{({ connectionId }) => <NetworksView connectionId={connectionId} />}</ConnectionGate>;
}

function NetworksView({ connectionId }: { connectionId: string }) {
  const pg = useNetworksPage(connectionId);
  const { list } = pg;

  return (
    <div className="relative flex h-full min-h-0">
      <div className="min-w-0 flex-1 overflow-auto p-5">
        <PageHeader
          title="Networks"
          subtitle="Docker networks on this Engine"
          actions={
            <>
              <Button size="sm" onClick={() => pg.setCreateOpen(true)}>
                <Plus className="size-4" /> Create Network
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button size="icon" variant="outline" className="size-8" aria-label="More options" title="More options">
                    <MoreHorizontal className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onSelect={() => void list.refetch()}><RefreshCw /> Refresh</DropdownMenuItem>
                  <DropdownMenuItem variant="destructive" disabled={pg.pruning} onSelect={() => pg.setPruneOpen(true)}>
                    <Trash2 /> Prune unused networks…
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          }
        />

        {list.isLoading ? (
          <Skeleton className="h-72 w-full rounded-[10px]" />
        ) : list.error ? (
          <EmptyState icon={<Activity />} title="Could not load networks">
            <p className="text-destructive">{String(list.error)}</p>
            <Button className="mt-3" onClick={() => void list.refetch()}>Retry</Button>
          </EmptyState>
        ) : (
          <TableCard>
            <TableToolbar
              selection={
                pg.selected.size > 0 ? (
                  <SelectionBar
                    count={pg.selected.size}
                    onClear={() => pg.setSelected(new Set())}
                    note={pg.removable.length < pg.selected.size ? "Predefined networks cannot be removed and are skipped" : undefined}
                  >
                    <Button
                      size="sm"
                      variant="outline"
                      className="border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive"
                      disabled={pg.removable.length === 0 || pg.removing}
                      onClick={() => pg.setConfirmRemove(pg.removable)}
                    >
                      <Trash2 className="size-3.5" /> Remove
                    </Button>
                  </SelectionBar>
                ) : undefined
              }
            >
              <UsageToolbar search={pg.search} onSearch={pg.setSearch} filter={pg.filter} onFilter={pg.setFilter} counts={pg.counts} placeholder="Search networks, drivers, subnets" />
            </TableToolbar>
            {pg.networks.length === 0 ? (
              <EmptyState icon={<NetworkIcon />} title="No networks on this Engine">
                <p>The Engine reports no networks.</p>
              </EmptyState>
            ) : pg.visible.length === 0 ? (
              <EmptyState icon={<NetworkIcon />} title="No matching networks">
                <Button variant="outline" onClick={() => { pg.setSearch(""); pg.setFilter("all"); }}>Clear filters</Button>
              </EmptyState>
            ) : (
              <NetworksTable
                rows={pg.visible}
                selected={pg.selected}
                inspected={pg.inspected}
                allVisibleSelected={pg.allVisibleSelected}
                onToggleAll={pg.toggleAllVisible}
                onToggle={pg.toggle}
                onInspect={pg.setInspected}
                onRemove={(n) => pg.setConfirmRemove([n])}
              />
            )}
          </TableCard>
        )}
      </div>

      {pg.inspectedNetwork && (
        <NetworkInspector
          key={pg.inspectedNetwork.id}
          connectionId={connectionId}
          network={pg.inspectedNetwork}
          busy={pg.removing}
          onClose={() => pg.setInspected(null)}
          onRemove={(n) => pg.setConfirmRemove([n])}
        />
      )}

      <CreateNetworkDialog connectionId={connectionId} open={pg.createOpen} onOpenChange={pg.setCreateOpen} />
      <NetworkConfirmDialogs
        confirmRemove={pg.confirmRemove}
        onCancelRemove={() => pg.setConfirmRemove(null)}
        onRemove={(t) => void pg.doRemove(t)}
        pruneOpen={pg.pruneOpen}
        onPruneOpenChange={pg.setPruneOpen}
        unused={pg.unused}
        onPrune={() => void pg.startPrune()}
      />
    </div>
  );
}
