import { Activity, Download, Hammer, Layers, MoreHorizontal, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ConnectionGate } from "@/components/shared/ConnectionGate";
import { EmptyState } from "@/components/shared/EmptyState";
import { PageHeader } from "@/components/shared/PageHeader";
import { SelectionBar } from "@/components/shared/SelectionBar";
import { TableCard, TableToolbar } from "@/components/shared/TableCard";
import CreateContainerDialog from "@/components/containers/CreateContainerDialog";
import { formatBytes } from "@/components/containers/format";
import { ImagesTable } from "@/components/images/ImagesTable";
import { ImageInspector } from "@/components/images/ImageInspector";
import { ImagesToolbar } from "@/components/images/ImagesToolbar";
import { CheckUpdatesButton } from "@/components/images/update/CheckUpdatesButton";
import { UpdateHost } from "@/components/images/update/UpdateHost";
import { ImageConfirmDialogs } from "@/components/images/ImageConfirmDialogs";
import { PullImageDialog } from "@/components/images/PullImageDialog";
import { BuildImageDialog } from "@/components/images/BuildImageDialog";
import { PushImageDialog } from "@/components/images/PushImageDialog";
import { TagImageDialog } from "@/components/images/TagImageDialog";
import { useImagesPage } from "@/components/images/useImagesPage";
import { useActiveConnection } from "@/hooks/useActiveConnection";

export default function ImagesPage() {
  return <ConnectionGate>{({ connectionId }) => <ImagesView connectionId={connectionId} />}</ConnectionGate>;
}

function ImagesView({ connectionId }: { connectionId: string }) {
  const { profile } = useActiveConnection();
  const pg = useImagesPage(connectionId);
  const { list, summary, selectedRows, transfers } = pg;
  const engineName = profile?.name ?? "the active Engine";
  const hiddenSelected = selectedRows.some((r) => !pg.visible.includes(r));

  const subtitle = list.data ? (
    <span aria-live="polite">
      {summary.images} image{summary.images === 1 ? "" : "s"}
      {summary.tagRows !== summary.images && ` (${summary.tagRows} references)`} ·{" "}
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="cursor-default underline decoration-dotted underline-offset-2">{formatBytes(summary.totalSize)} total</span>
        </TooltipTrigger>
        <TooltipContent>Sum of image sizes as reported by the Engine. Images that share layers are counted separately, so actual disk use can be lower.</TooltipContent>
      </Tooltip>{" "}
      · {summary.unused} unused
    </span>
  ) : (
    "Images stored on this Engine"
  );

  return (
    <div className="relative flex h-full min-h-0">
      <div className="min-w-0 flex-1 overflow-auto p-5">
        <PageHeader
          title="Images"
          subtitle={subtitle}
          actions={
            <>
              <Button size="sm" onClick={() => pg.setPullOpen(true)}>
                <Download className="size-4" /> Pull Image
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button size="icon" variant="outline" className="size-8" aria-label="More options" title="More options">
                    <MoreHorizontal className="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onSelect={() => pg.setBuildOpen(true)}><Hammer /> Build Image…</DropdownMenuItem>
                  <DropdownMenuItem disabled={transfers.busy} onSelect={() => void transfers.loadImage()}>
                    <Download className="rotate-180" /> Load Image…
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          }
        />

        {list.isLoading ? (
          <Skeleton className="h-72 w-full rounded-[10px]" />
        ) : list.error ? (
          <EmptyState icon={<Activity />} title="Could not load images">
            <p className="text-destructive">{String(list.error)}</p>
            <Button className="mt-3" onClick={() => void list.refetch()}>Retry</Button>
          </EmptyState>
        ) : (
          <TableCard>
            <TableToolbar
              selection={
                selectedRows.length > 0 ? (
                  <SelectionBar
                    count={selectedRows.length}
                    onClear={() => pg.setSelected(new Set())}
                    note={hiddenSelected ? "includes images hidden by filters" : undefined}
                  >
                    <Button size="sm" variant="outline" disabled={transfers.busy} onClick={() => void transfers.saveImages(selectedRows)}>
                      <Download className="size-3.5" /> Save…
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive"
                      disabled={pg.removing}
                      onClick={pg.requestRemoveSelected}
                    >
                      <Trash2 className="size-3.5" /> Remove
                    </Button>
                  </SelectionBar>
                ) : undefined
              }
            >
              <ImagesToolbar
                filter={pg.filter}
                onFilter={pg.setFilter}
                counts={pg.counts}
                query={pg.query}
                onQuery={pg.setQuery}
                danglingCount={pg.danglingRows.length}
                pruning={pg.pruning}
                onPrune={() => pg.setConfirmPrune(true)}
                refreshing={list.isFetching}
                onRefresh={() => void list.refetch()}
                extra={<CheckUpdatesButton connectionId={connectionId} rows={list.data ?? []} />}
              />
            </TableToolbar>
            {pg.rows.length === 0 ? (
              <EmptyState icon={<Layers />} title="No images on this Engine">
                <p>Pull an image to get started.</p>
                <Button className="mt-3" onClick={() => pg.setPullOpen(true)}>Pull Image</Button>
              </EmptyState>
            ) : pg.visible.length === 0 ? (
              <EmptyState icon={<Layers />} title="No matching images">
                <Button variant="outline" onClick={() => { pg.setQuery(""); pg.setFilter("all"); }}>Clear filters</Button>
              </EmptyState>
            ) : (
              <ImagesTable
                rows={pg.visible}
                selected={pg.selected}
                inspectedKey={pg.inspectedKey}
                onToggleSelect={pg.onToggleSelect}
                onToggleSelectAll={pg.onToggleSelectAll}
                onInspect={pg.onInspect}
                onCreateContainer={pg.onCreate}
                onTag={pg.onTag}
                onRemove={pg.onRemoveRow}
                onPush={pg.onPush}
                onSave={pg.onSaveRow}
              />
            )}
          </TableCard>
        )}
      </div>

      {pg.inspected && (
        <ImageInspector
          key={pg.inspected.key}
          connectionId={connectionId}
          row={pg.inspected}
          width={pg.inspectorWidth}
          onResize={pg.setInspectorWidth}
          onClose={pg.closeInspector}
        />
      )}

      <UpdateHost />
      <PullImageDialog connectionId={connectionId} engineName={engineName} open={pg.pullOpen} onOpenChange={pg.setPullOpen} />
      <BuildImageDialog connectionId={connectionId} engineName={engineName} open={pg.buildOpen} onOpenChange={pg.setBuildOpen} />
      <PushImageDialog connectionId={connectionId} engineName={engineName} image={pg.pushTarget} onClose={() => pg.setPushTarget(null)} />
      <TagImageDialog connectionId={connectionId} image={pg.tagTarget} onClose={() => pg.setTagTarget(null)} />
      <CreateContainerDialog
        connectionId={connectionId}
        open={!!pg.createFor}
        onOpenChange={(o) => !o && pg.setCreateFor(null)}
        image={pg.createFor?.ref}
      />
      <ImageConfirmDialogs
        confirmRemove={pg.confirmRemove}
        onCancelRemove={() => pg.setConfirmRemove(null)}
        force={pg.force}
        onForceChange={pg.setForce}
        onRemove={() => void pg.doRemove()}
        confirmPrune={pg.confirmPrune}
        onPruneOpenChange={pg.setConfirmPrune}
        danglingRows={pg.danglingRows}
        onPrune={() => void pg.doPrune()}
      />
    </div>
  );
}
