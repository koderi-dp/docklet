import { useCallback, useMemo, useRef } from "react";
import { PanelLeftOpen, ScrollText } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/EmptyState";
import { PageHeader } from "@/components/shared/PageHeader";
import { useContainersQuery } from "@/hooks/useContainers";
import { useElementWidth } from "@/hooks/useElementWidth";
import { OVERALL_CAP, PER_CONTAINER_CAP } from "@/lib/logMerge";
import { MAX_STREAMS } from "@/lib/logSources";
import type { TickedItem } from "@/lib/logPanes";
import { useUI } from "@/stores/uiStore";
import { useLogFilterStore } from "@/stores/logFilterStore";
import { ContainerPicker } from "./ContainerPicker";
import { LogView } from "./LogView";
import { LayoutSwitch } from "./split/LayoutSwitch";
import { SplitLayout } from "./split/SplitLayout";
import { usePaneAssignments } from "./split/usePaneAssignments";
import { useFilteredLogs } from "./useFilteredLogs";
import { useLogFilter } from "./useLogFilter";
import { useLogSelection } from "./useLogSelection";
import { useMergedLogStreams } from "./useMergedLogStreams";

/** Lines fetched per container when a stream opens: the user's log tail setting, kept modest for many containers. */
function initialTail(setting: number): number {
  return setting > 0 ? Math.min(setting, 1000) : 500;
}

/** Below this width of the log area, side-by-side panes are stacked instead. */
const NARROW_PX = 760;

/** The Logs screen: pick containers on the left, read one merged, filterable stream on the right. */
export function LogsScreen({ connectionId }: { connectionId: string }) {
  const list = useContainersQuery(connectionId, true);
  const containers = list.data ?? undefined;
  const onLimit = useCallback(() => toast.warning(`At most ${MAX_STREAMS} containers can stream at once`), []);
  const sel = useLogSelection(containers, onLimit);
  const logTail = useUI((s) => s.logTail);
  const collapsed = useLogFilterStore((s) => s.panelCollapsed);
  const setCollapsed = useLogFilterStore((s) => s.setPanelCollapsed);

  const targets = useMemo(
    () =>
      (containers ?? [])
        .filter((c) => sel.ticked.has(c.id))
        .map((c) => ({ id: c.id, name: c.name }))
        .sort((a, b) => a.id.localeCompare(b.id)),
    [containers, sel.ticked],
  );
  const streams = useMergedLogStreams(connectionId, targets, initialTail(logTail));
  const items = useMemo<TickedItem[]>(
    () => (containers ?? []).filter((c) => sel.ticked.has(c.id)).map((c) => ({ name: c.name, project: c.composeProject || null })),
    [containers, sel.ticked],
  );
  const split = usePaneAssignments(connectionId, items, sel.ready);
  const area = useRef<HTMLDivElement>(null);
  const areaWidth = useElementWidth(area);
  const narrow = areaWidth > 0 && areaWidth < NARROW_PX;
  const drawn = narrow && split.layout === "cols" ? "stack2" : narrow && split.layout === "grid" ? "stack4" : split.layout;
  const filter = useLogFilter({ restore: true });
  const result = useFilteredLogs(streams.lines, filter);

  const tagWidth = Math.min(26, Math.max(8, ...targets.map((t) => t.name.length)));
  const failed = Object.values(streams.infos).filter((i) => i.status === "error").length;
  const connecting = Object.values(streams.infos).filter((i) => i.status === "connecting").length;
  const total = streams.lines.length;

  const status = (
    <span>
      <span className="text-foreground tabular-nums">{result.visible.length.toLocaleString()}</span> of <span className="tabular-nums">{total.toLocaleString()}</span> lines
      {` · ${targets.length} container${targets.length === 1 ? "" : "s"}`}
      {streams.paused ? ` · Paused${streams.unseen ? ` · ${streams.unseen.toLocaleString()} new` : ""}` : connecting ? " · Connecting…" : " · Live"}
      {failed > 0 && <span className="text-destructive">{` · ${failed} stream${failed === 1 ? "" : "s"} failed`}</span>}
      {streams.dropped > 0 && (
        <span className="text-warning" title={`Each container keeps its latest ${PER_CONTAINER_CAP.toLocaleString()} lines, ${OVERALL_CAP.toLocaleString()} in total`}>
          {` · ${streams.dropped.toLocaleString()} older lines dropped`}
        </span>
      )}
    </span>
  );

  let body;
  if (list.isPending) body = <Skeleton className="h-full min-h-[240px] w-full" />;
  else if (list.isError)
    body = (
      <EmptyState title="Could not load containers">
        <p>{String(list.error)}</p>
        <Button className="mt-3" onClick={() => void list.refetch()}>
          Retry
        </Button>
      </EmptyState>
    );
  else if ((containers ?? []).length === 0)
    body = (
      <EmptyState icon={<ScrollText />} title="No containers on this engine">
        <p>Logs appear here once a container exists.</p>
      </EmptyState>
    );
  else if (targets.length === 0)
    body = (
      <EmptyState icon={<ScrollText />} title="Tick containers on the left to see their logs">
        <p>Pick a whole Compose project or single containers. Their output is merged into one stream, ordered by time.</p>
      </EmptyState>
    );
  else
    body = (
      <LogView
        className="h-full"
        total={total}
        filter={filter}
        result={result}
        showContainer
        containerNames={targets.map((t) => t.name)}
        tagWidth={tagWidth}
        status={status}
        paused={streams.paused}
        onPause={() => streams.setPaused(!streams.paused)}
        canPause
        extra={<LayoutSwitch layout={split.layout} onChange={split.setLayout} narrow={narrow} />}
        renderBody={
          split.layout === "single"
            ? undefined
            : (ctx) => (
                <SplitLayout
                  key={split.layout}
                  layout={drawn}
                  panes={split.panes}
                  items={items}
                  visible={result.visible}
                  highlights={result.highlights}
                  wrap={ctx.wrap}
                  timestamps={ctx.timestamps}
                  actions={ctx.actions}
                  streamEmpty={ctx.empty}
                  note={split.note}
                  sizes={split.sizes}
                  onSizes={split.setSizes}
                  onPaneAll={split.setPaneAll}
                  onPaneNames={split.setPaneNames}
                  downloadName="docklet-logs"
                />
              )
        }
        onClear={streams.clear}
        emptyNoLines={<p className="p-3 text-muted-foreground">{connecting > 0 ? "Connecting to log streams…" : "No log output yet. New lines appear as the containers write them."}</p>}
        ariaLabel="Merged container logs"
        downloadName="docklet-logs"
      />
    );

  return (
    <div className="flex h-full min-h-0 flex-col p-5">
      <PageHeader title="Logs" subtitle="One merged stream from the containers you tick" />
      <div className="flex min-h-0 flex-1 gap-3">
        {collapsed ? (
          <div className="flex w-10 shrink-0 flex-col items-center gap-2 rounded-lg border border-border bg-card py-2">
            <Button variant="ghost" size="icon-sm" className="size-8" aria-label="Show container list" title="Show container list" onClick={() => setCollapsed(false)}>
              <PanelLeftOpen className="size-4" />
            </Button>
            <span className="text-xs text-muted-foreground tabular-nums" title={`${sel.ticked.size} containers selected`}>
              {sel.ticked.size}
            </span>
          </div>
        ) : (
          containers && (
            <ContainerPicker containers={containers} ticked={sel.ticked} onSet={sel.setMany} onClear={sel.clear} infos={streams.infos} capped={sel.capped} onCollapse={() => setCollapsed(true)} />
          )
        )}
        <div ref={area} className="min-w-0 flex-1">
          {body}
        </div>
      </div>
    </div>
  );
}
