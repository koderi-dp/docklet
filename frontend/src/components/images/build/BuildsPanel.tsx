import { useEffect, useMemo, useState } from "react";
import { Copy, Eye, Hammer, Layers, MoreHorizontal, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { OperationStatus, type BuildRecord, type ImageRow, type Operation } from "@bindings/model";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/EmptyState";
import { TableCard, TableToolbar } from "@/components/shared/TableCard";
import { formatDateTime } from "@/components/containers/format";
import { useOperations } from "@/hooks/useOperations";
import { copyToClipboard } from "@/lib/clipboard";
import { errorMessage } from "@/lib/errors";
import { cn } from "@/lib/utils";
import { BuildDetailsDialog } from "./BuildDetailsDialog";
import { BuildResultBadge } from "./BuildResultBadge";
import { summaryLine, formatDuration } from "./buildFormat";
import { useBuildHistory, useBuildProgress, useClearBuilds, useRemoveBuild } from "./useBuildApi";

interface Props {
  connectionId: string;
  images: ImageRow[];
  /** Opens the empty Build dialog. */
  onBuild: () => void;
  onBuildAgain: (r: BuildRecord) => void;
  onShowImage: (r: BuildRecord) => void;
}

const TH = "h-10 whitespace-nowrap px-3 font-medium";
const TD = "px-3";

function useRunningBuilds(connectionId: string): Operation[] {
  const { operations } = useOperations();
  return useMemo(() => operations.filter((o) => o.kind === "image.build" && o.connectionId === connectionId && o.status === OperationStatus.OpRunning), [operations, connectionId]);
}

function useTick(active: boolean) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!active) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [active]);
  return now;
}

function RowMenu({ r, canShow, onOpen, onAgain, onShow, onRemove }: { r: BuildRecord; canShow: boolean; onOpen: () => void; onAgain: () => void; onShow: () => void; onRemove: () => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="size-7" aria-label="Build actions" title="Build actions" onClick={(e) => e.stopPropagation()}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
        <DropdownMenuItem onSelect={onOpen}>
          <Eye /> Details
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onAgain}>
          <Hammer /> Build again
        </DropdownMenuItem>
        <DropdownMenuItem disabled={!canShow} title={canShow ? undefined : "The image no longer exists on the Engine"} onSelect={onShow}>
          <Layers /> Show image
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => copyToClipboard(r.command, "Command")}>
          <Copy /> Copy command
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={onRemove}>
          <Trash2 /> Remove from history
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

const rowCls = "h-[42px] cursor-pointer border-b border-border/60 outline-none transition-colors hover:bg-muted/50 focus-visible:bg-muted/60 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/60";

function LiveRow({ op, onOpen }: { op: Operation; onOpen: () => void }) {
  const now = useTick(true);
  const q = useBuildProgress(op.id, true);
  const p = q.data;
  return (
    <tr tabIndex={0} onClick={onOpen} onKeyDown={(e) => e.key === "Enter" && e.target === e.currentTarget && onOpen()} className={cn(rowCls, "bg-info/5")}>
      <td className={cn(TD, "whitespace-nowrap")}>{formatDateTime(new Date(op.startedAt))}</td>
      <td className={TD}>
        <span className="block max-w-[320px] truncate font-medium">{op.resource}</span>
      </td>
      <td className={TD}>
        <BuildResultBadge result="running" />
      </td>
      <td className={cn(TD, "whitespace-nowrap tabular-nums")}>{formatDuration(now - op.startedAt)}</td>
      <td className={cn(TD, "text-muted-foreground")}>{p ? "BuildKit" : "—"}</td>
      <td className={cn(TD, "whitespace-nowrap tabular-nums text-muted-foreground")}>{p && p.total > 0 ? summaryLine(p.done, p.total, p.cached) : "—"}</td>
      <td className={TD} />
    </tr>
  );
}

/** "Builds" view of the Images page: builds running now and the history of builds started from Docklet. */
export function BuildsPanel({ connectionId, images, onBuild, onBuildAgain, onShowImage }: Props) {
  const history = useBuildHistory(connectionId);
  const running = useRunningBuilds(connectionId);
  const { cancel } = useOperations();
  const remove = useRemoveBuild(connectionId);
  const clear = useClearBuilds(connectionId);
  const [openId, setOpenId] = useState<string | null>(null);
  const [openLive, setOpenLive] = useState<string | null>(null);
  const [confirmRemove, setConfirmRemove] = useState<BuildRecord | null>(null);
  const [confirmClear, setConfirmClear] = useState(false);
  const { operations } = useOperations();

  const records = history.data ?? [];
  const imageIds = useMemo(() => new Set(images.map((i) => i.id)), [images]);
  const exists = (r: BuildRecord) => !!r.imageId && imageIds.has(r.imageId);
  const record = records.find((r) => r.id === openId) ?? null;
  const live = operations.find((o) => o.id === openLive) ?? null;

  const doRemove = async (r: BuildRecord) => {
    try {
      await remove.mutateAsync(r.id);
      setConfirmRemove(null);
      if (openId === r.id) setOpenId(null);
      toast.success("Removed from build history");
    } catch (e) {
      toast.error("Could not remove the build", { description: errorMessage(e), duration: Infinity, closeButton: true });
    }
  };
  const doClear = async () => {
    try {
      await clear.mutateAsync();
      setConfirmClear(false);
      toast.success("Build history cleared");
    } catch (e) {
      toast.error("Could not clear the build history", { description: errorMessage(e), duration: Infinity, closeButton: true });
    }
  };

  return (
    <>
      <TableCard>
        <TableToolbar>
          <p className="min-w-0 flex-1 truncate text-xs text-muted-foreground">Builds started from Docklet on this computer. The last 50 per connection are kept; argument values are never stored.</p>
          <Button size="sm" variant="outline" disabled={records.length === 0} onClick={() => setConfirmClear(true)}>
            <Trash2 className="size-3.5" /> Clear history
          </Button>
          <Button size="sm" onClick={onBuild}>
            <Hammer className="size-3.5" /> Build Image
          </Button>
        </TableToolbar>
        {history.isLoading ? (
          <Skeleton className="m-3 h-40 rounded-[10px]" />
        ) : history.error ? (
          <EmptyState icon={<Hammer />} title="Could not load the build history">
            <p className="text-destructive">{errorMessage(history.error)}</p>
            <Button className="mt-3" onClick={() => void history.refetch()}>
              Retry
            </Button>
          </EmptyState>
        ) : records.length === 0 && running.length === 0 ? (
          <EmptyState icon={<Hammer />} title="No builds yet">
            <p>Builds you start here are listed with their steps, options and result.</p>
            <Button className="mt-3" onClick={onBuild}>
              <Hammer className="size-4" /> Build Image
            </Button>
          </EmptyState>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] border-collapse text-[13px]">
              <thead>
                <tr className="border-b border-border text-left text-xs font-medium text-muted-foreground">
                  <th scope="col" className={TH}>When</th>
                  <th scope="col" className={TH}>Tags</th>
                  <th scope="col" className={TH}>Result</th>
                  <th scope="col" className={TH}>Duration</th>
                  <th scope="col" className={TH}>Builder</th>
                  <th scope="col" className={TH}>Steps</th>
                  <th scope="col" className={cn(TH, "w-10")}>
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {running.map((op) => (
                  <LiveRow key={op.id} op={op} onOpen={() => setOpenLive(op.id)} />
                ))}
                {records.map((r) => (
                  <tr
                    key={r.id}
                    tabIndex={0}
                    onClick={() => setOpenId(r.id)}
                    onKeyDown={(e) => {
                      if (e.target === e.currentTarget && (e.key === "Enter" || e.key === " ")) {
                        e.preventDefault();
                        setOpenId(r.id);
                      }
                    }}
                    className={rowCls}
                  >
                    <td className={cn(TD, "whitespace-nowrap")}>{formatDateTime(new Date(r.startedAt))}</td>
                    <td className={TD}>
                      <span className="block max-w-[320px] truncate font-medium" title={(r.tags ?? []).join(", ")}>
                        {r.tags?.length ? r.tags[0] : <span className="font-normal text-muted-foreground">Untagged</span>}
                        {(r.tags?.length ?? 0) > 1 && <span className="font-normal text-muted-foreground"> +{(r.tags?.length ?? 1) - 1}</span>}
                      </span>
                    </td>
                    <td className={TD}>
                      <BuildResultBadge result={r.result} />
                    </td>
                    <td className={cn(TD, "whitespace-nowrap tabular-nums")}>{formatDuration(r.durationMs)}</td>
                    <td className={cn(TD, "whitespace-nowrap")}>{r.builder === "buildkit" ? "BuildKit" : "Classic"}</td>
                    <td className={cn(TD, "whitespace-nowrap tabular-nums")}>
                      {r.stepsTotal > 0 ? (
                        <>
                          {r.stepsTotal}
                          {r.stepsCached > 0 && <span className="text-muted-foreground"> · {r.stepsCached} cached</span>}
                        </>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </td>
                    <td className={TD}>
                      <div className="flex justify-end">
                        <RowMenu r={r} canShow={exists(r)} onOpen={() => setOpenId(r.id)} onAgain={() => onBuildAgain(r)} onShow={() => onShowImage(r)} onRemove={() => setConfirmRemove(r)} />
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </TableCard>

      <BuildDetailsDialog
        record={record}
        live={record ? null : live}
        imageExists={!!record && exists(record)}
        onClose={() => {
          setOpenId(null);
          setOpenLive(null);
        }}
        onBuildAgain={(r) => {
          setOpenId(null);
          onBuildAgain(r);
        }}
        onShowImage={(r) => {
          setOpenId(null);
          onShowImage(r);
        }}
        onRemove={setConfirmRemove}
        onCancel={(op) => void cancel(op.id)}
      />

      <AlertDialog open={!!confirmRemove} onOpenChange={(o) => !o && setConfirmRemove(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove this build from the history?</AlertDialogTitle>
            <AlertDialogDescription>
              Only Docklet&apos;s record of the build ({confirmRemove?.tags?.[0] ?? "untagged"}, {confirmRemove ? formatDateTime(new Date(confirmRemove.startedAt)) : ""}) is deleted. The image on the Engine is not touched.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => confirmRemove && void doRemove(confirmRemove)}>
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={confirmClear} onOpenChange={setConfirmClear}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Clear the build history?</AlertDialogTitle>
            <AlertDialogDescription>
              Deletes Docklet&apos;s records of {records.length} build{records.length === 1 ? "" : "s"} on this connection. Images on the Engine are not touched. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => void doClear()}>
              Clear history
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
